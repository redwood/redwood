package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"sync"

	git "github.com/brynbellomy/git2go"
	"golang.org/x/crypto/sha3"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm/braidhttp"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

func push(srcRefName string, destRefName string, client *braidhttp.LightClient) error {
	logf("%v -> %v", srcRefName, destRefName)

	err := ensureStateTree(client)
	if err != nil {
		return err
	}

	force := strings.HasPrefix(srcRefName, "+")
	if force {
		srcRefName = srcRefName[1:]
	}

	ref, err := repo.References.Dwim(srcRefName)
	if err != nil {
		return errors.WithStack(err)
	}
	headCommitID := ref.Target()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		chErr          = make(chan error)
		claimedCommits sync.Map
		claimedBlobs   sync.Map
		wg             sync.WaitGroup
	)

	go func() {
		select {
		case err, noErr := <-chErr:
			if !noErr {
				cancel()
				logf("error: %v", err)
				return
			}
		case <-ctx.Done():
		}
	}()

	stack := []*git.Oid{headCommitID}
	parentTxID := tree.GenesisTxID
	for len(stack) > 0 {
		commitID := stack[0]
		stack = stack[1:]

		has, err := remoteHasCommit(commitID, client)
		if err != nil {
			return err
		}

		if !has {
			_, existed := claimedCommits.LoadOrStore(commitID.String(), true)
			if existed {
				continue
			}

			wg.Add(1)
			txID := state.RandomVersion()
			err = pushCommit(ctx, commitID, txID, parentTxID, destRefName, client, chErr, &claimedBlobs, &wg)
			if err != nil {
				return err
			}
			parentTxID = txID
		}

		parentIDs, err := getParentIDs(commitID)
		if err != nil {
			return err
		}
		stack = append(stack, parentIDs...)
	}
	wg.Wait()

	return updateRef(destRefName, headCommitID, parentTxID, client)
}

func ensureStateTree(client *braidhttp.LightClient) error {
	_, _, _, err := client.Get(StateURI, nil, nil, nil, true)
	if errors.Cause(err) == errors.Err404 {
		// Create the initial state tree
		tx := tree.Tx{
			ID:       tree.GenesisTxID,
			StateURI: StateURI,
			From:     sigkeys.Address(),
			Parents:  nil,
			Patches: []tree.Patch{{
				ValueJSON: mustJSON(M{
					"Merge-Type": M{
						"Content-Type": "resolver/dumb",
						"value":        M{},
					},
					"Validator": M{
						"Content-Type": "validator/permissions",
						"value": M{
							"*": M{
								"^.*$": M{
									"write": true,
								},
							},
						},
					},
					"refs": M{
						"heads": M{},
					},
					"commits": M{},
				}),
			}},
		}
		err := client.Put(context.Background(), tx)
		if err != nil {
			return err
		}
		logf("initialized state tree for " + StateURI)

	} else if err != nil {
		return err
	}
	return nil
}

func remoteHasCommit(commitID *git.Oid, client *braidhttp.LightClient) (bool, error) {
	keypath := state.Keypath("commits").Pushs(commitID.String())
	stateReader, _, _, err := client.Get(StateURI, nil, keypath, nil, true)
	if errors.Cause(err) == errors.Err404 {
		return false, nil
	} else if err != nil {
		return false, err
	}
	var val interface{}
	err = json.NewDecoder(stateReader).Decode(&val)
	if err != nil {
		return false, err
	}
	return val != nil, nil
}

func getParentIDs(childCommitID *git.Oid) ([]*git.Oid, error) {
	commit, err := repo.LookupCommit(childCommitID)
	if err != nil {
		return nil, err
	}

	var parentIDs []*git.Oid
	parentCount := commit.ParentCount()
	for i := uint(0); i < parentCount; i++ {
		parentIDs = append(parentIDs, commit.ParentId(i))
	}
	return parentIDs, nil
}

type commitFile struct {
	oid        *git.Oid
	objectType git.ObjectType
	fullPath   string
	mode       git.Filemode
}

type uploadedFile struct {
	name        string
	contentType string
	size        uint64
	gitOid      string
	hash        types.Hash
	mode        git.Filemode
}

func pushCommit(
	ctx context.Context,
	commitID *git.Oid,
	txID state.Version,
	parentTxID state.Version,
	destRefName string,
	client *braidhttp.LightClient,
	chErr chan error,
	claimedBlobs *sync.Map,
	wg *sync.WaitGroup,
) (err error) {
	defer handleError(ctx, &err, chErr)
	defer wg.Done()

	commit, err := repo.LookupCommit(commitID)
	if err != nil {
		logf("ERR: %v", err)
		return
	}
	defer commit.Free()

	var (
		chFiles    = make(chan commitFile)
		chUploaded = make(chan uploadedFile)
		innerWg    sync.WaitGroup
	)

	innerWg.Add(3)
	go filesInCommit(ctx, commit, chFiles, chErr, &innerWg)
	go uploadFilesInCommit(ctx, client, chFiles, chUploaded, chErr, claimedBlobs, &innerWg)
	go sendCommitTx(ctx, commit, txID, parentTxID, client, chUploaded, chErr, &innerWg)
	innerWg.Wait()

	return nil
}

func filesInCommit(ctx context.Context, commit *git.Commit, chFiles chan<- commitFile, chErr chan<- error, wg *sync.WaitGroup) (err error) {
	defer wg.Done()
	defer close(chFiles)
	defer handleError(ctx, &err, chErr)

	commitTree, err := commit.Tree()
	if err != nil {
		return
	}
	defer commitTree.Free()

	return commitTree.Walk(func(rootPath string, entry *git.TreeEntry) int {
		select {
		case <-ctx.Done():
			return -1
		case chFiles <- commitFile{entry.Id, entry.Type, filepath.Join(rootPath, entry.Name), entry.Filemode}:
		}
		return 0
	})
}

func uploadFilesInCommit(
	ctx context.Context,
	client *braidhttp.LightClient,
	chFiles <-chan commitFile,
	chUploaded chan<- uploadedFile,
	chErr chan<- error,
	claimedBlobs *sync.Map,
	wg *sync.WaitGroup,
) (err error) {
	defer wg.Done()
	defer close(chUploaded)
	defer handleError(ctx, &err, chErr)

	var innerWg sync.WaitGroup

	for commitFile := range chFiles {
		if commitFile.objectType == git.ObjectBlob {
			uploadBlob(ctx, commitFile, client, &innerWg, chUploaded, chErr)
		} else if commitFile.objectType == git.ObjectCommit {
			select {
			case <-ctx.Done():
				return
			case chUploaded <- uploadedFile{commitFile.fullPath, "", 0, commitFile.oid.String(), types.Hash{}, commitFile.mode}:
			}
		}
	}
	innerWg.Wait()
	return
}

func uploadBlob(
	ctx context.Context,
	commitFile commitFile,
	client *braidhttp.LightClient,
	innerWg *sync.WaitGroup,
	chUploaded chan<- uploadedFile,
	chErr chan<- error,
) {
	gitBlob, err := repo.LookupBlob(commitFile.oid)
	if err != nil {
		logf("ERR: %v", err)
		return
	}

	innerWg.Add(1)
	go func() (err error) {
		defer innerWg.Done()
		defer handleError(ctx, &err, chErr)

		data := gitBlob.Contents()

		var contentType string
		contentType, err = utils.SniffContentType(commitFile.fullPath, bytes.NewBuffer(data))
		if err != nil {
			return
		}

		sha3Hasher := sha3.NewLegacyKeccak256()
		_, err = io.Copy(sha3Hasher, bytes.NewReader(data))
		if err != nil {
			return
		}
		localSHA3Bytes := sha3Hasher.Sum(nil)

		var blobID blob.ID
		blobID.HashAlg = types.SHA3
		copy(blobID.Hash[:], localSHA3Bytes)

		have, err := client.HaveBlob(blobID)
		if err != nil {
			return
		}

		var sha3Hash types.Hash
		if !have {
			var resp braidhttp.StoreBlobResponse
			resp, err = client.StoreBlob(bytes.NewReader(data))
			if err != nil {
				return
			}
			logf("blob " + resp.SHA3.Hex())
			sha3Hash = resp.SHA3
		} else {
			sha3Hash = blobID.Hash
		}
		select {
		case <-ctx.Done():
			return
		case chUploaded <- uploadedFile{commitFile.fullPath, contentType, uint64(len(data)), commitFile.oid.String(), sha3Hash, commitFile.mode}:
		}
		return
	}()
}

func sendCommitTx(
	ctx context.Context,
	gitCommit *git.Commit,
	txID state.Version,
	parentTxID state.Version,
	client *braidhttp.LightClient,
	chUploaded <-chan uploadedFile,
	chErr chan<- error,
	wg *sync.WaitGroup,
) (err error) {
	defer wg.Done()
	defer handleError(ctx, &err, chErr)

	var parentStrs []string
	if gitCommit.ParentCount() > 0 {
		for i := uint(0); i < gitCommit.ParentCount(); i++ {
			parentStr := gitCommit.ParentId(i).String()
			parentStrs = append(parentStrs, parentStr)
		}
	}

	commitHash := gitCommit.Id().String()
	fileTree := state.NewMemoryNode()
Loop:
	for {
		select {
		case uploadedFile, open := <-chUploaded:
			if !open {
				break Loop
			}
			if uploadedFile.mode == git.FilemodeCommit {
				// Submodule
				err = fileTree.Set(state.Keypath(uploadedFile.name), nil, M{
					"Content-Type": "submodule",
					"value":        uploadedFile.gitOid,
					"mode":         int(uploadedFile.mode),
				})
			} else {
				// Regular file
				err = fileTree.Set(state.Keypath(uploadedFile.name), nil, M{
					"Content-Type":   "link",
					"Content-Length": uploadedFile.size,
					"value":          "blob:sha3:" + uploadedFile.hash.Hex(),
					"sha1":           uploadedFile.gitOid,
					"mode":           int(uploadedFile.mode),
				})
			}
			if err != nil {
				return
			}

		case <-ctx.Done():
			return
		}
	}

	sig, _, err := gitCommit.ExtractSignature()
	if git.IsErrorClass(err, git.ErrClassObject) {
		// This simply means no signature
	} else if err != nil {
		return err
	}

	tx := tree.Tx{
		ID:       txID,
		StateURI: StateURI,
		From:     sigkeys.Address(),
		Parents:  []state.Version{parentTxID},
		Patches: []tree.Patch{
			{
				Keypath: state.Keypath("commits/" + commitHash),
				ValueJSON: mustJSON(Commit{
					Parents:   parentStrs,
					Message:   gitCommit.Message(),
					Timestamp: gitCommit.Time(),
					Author: AuthorCommitter{
						Name:      gitCommit.Author().Name,
						Email:     gitCommit.Author().Email,
						Timestamp: gitCommit.Author().When,
					},
					Committer: AuthorCommitter{
						Name:      gitCommit.Committer().Name,
						Email:     gitCommit.Committer().Email,
						Timestamp: gitCommit.Committer().When,
					},
					Files:     fileTree,
					Signature: sig,
				}),
			},
		},
	}

	err = client.Put(ctx, tx)
	if err != nil {
		return
	}
	logf("commit " + commitHash)
	return
}

func updateRef(destRefName string, commitID *git.Oid, parentTxID state.Version, client *braidhttp.LightClient) (err error) {
	defer errors.AddStack(&err)

	branchKeypath := RootKeypath.Push(state.Keypath(destRefName))

	tx := tree.Tx{
		ID:       state.RandomVersion(),
		StateURI: StateURI,
		From:     sigkeys.Address(),
		Parents:  []state.Version{parentTxID},
		Patches: []tree.Patch{
			{
				Keypath:   branchKeypath.Push(state.Keypath("HEAD")),
				ValueJSON: mustJSON(commitID.String()),
			},
			{
				Keypath: branchKeypath.Push(state.Keypath("worktree")),
				ValueJSON: mustJSON(map[string]interface{}{
					"Content-Type": "link",
					"value":        "state:" + StateURI + "/commits/" + commitID.String() + "/files",
				}),
			},
		},
	}

	err = client.Put(context.Background(), tx)
	if err != nil {
		return
	}
	logf("ref " + destRefName)
	return
}

func handleError(ctx context.Context, err *error, chErr chan<- error) {
	if *err != nil {
		select {
		case chErr <- *err:
		case <-ctx.Done():
		}
	}
}
