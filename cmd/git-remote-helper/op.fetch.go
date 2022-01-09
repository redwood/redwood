package main

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	git "github.com/brynbellomy/git2go"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm/braidhttp"
	"redwood.dev/utils"
)

func fetch(client *braidhttp.LightClient, headCommitHash string) (err error) {
	defer errors.AddStack(&err)

	odb, err := repo.Odb()
	if err != nil {
		logf("ERR: %v (commit %v)", err, headCommitHash)
		return
	}

	// Fetch just the ancestors of the requested commit
	// @@TODO: exclude anything we already have
	var (
		stack              = []string{headCommitHash}
		commits            = []Commit{}
		alreadySeenCommits = make(map[string]struct{})
	)
	for len(stack) > 0 {
		commitHash := stack[0]
		stack = stack[1:]

		var stateReader io.ReadCloser
		stateReader, _, _, err = client.Get(StateURI, nil, RootKeypath.Pushs("commits").Pushs(commitHash), nil, true)
		if err != nil {
			logf("ERR: %v (commit %v)", err, commitHash)
			return
		}
		defer stateReader.Close()

		var commit Commit
		commit.Files = state.NewMemoryNode()
		err = json.NewDecoder(stateReader).Decode(&commit)
		if err != nil {
			logf("ERR: %v (commit %v)", err, commitHash)
			return
		}
		commit.Hash = commitHash

		for _, pid := range commit.Parents {
			stack = append(stack, pid)
		}
		commits = append(commits, commit)
	}

	refs := &sync.Map{}

	// We have to fetch the commits in reverse, from oldest to newest,
	// because writing each one to disk requires having its parents
	for i := len(commits) - 1; i >= 0; i-- {
		commit := commits[i]

		if _, exists := alreadySeenCommits[commit.Hash]; exists {
			continue
		}
		alreadySeenCommits[commit.Hash] = struct{}{}

		err = fetchCommit(commit.Hash, commit, client, odb, refs)
		if err != nil {
			logf("ERR: %v (commit %v)", err, commit.Hash)
			return err
		}
	}

	return nil
}

type BlobEntry struct {
	oid  git.Oid
	size int64
}

func fetchCommit(commitHash string, commit Commit, client *braidhttp.LightClient, odb *git.Odb, refs *sync.Map) (err error) {
	defer errors.Annotate(&err, "fetchCommit")

	logf("fetch commit: %v", commitHash)

	idx, err := git.NewIndex()
	if err != nil {
		return errors.WithStack(err)
	}

	if commit.Files != nil {
		err = walkFiles(commit.Files, nil, func(node interface{}, fileData FileData, filePath state.Keypath) error {
			var refEntry BlobEntry
			if fileData.Mode == git.FilemodeCommit || fileData.ContentType == "submodule" {
				refEntry, err = createSubmodule(fileData)
			} else if fileData.ContentType == "link" && fileData.Value[:len("blob:")] == "blob:" {
				refEntry, err = createBlob(commitHash, filePath, fileData, client, odb)
			} else if fileData.Mode == git.FilemodeTree {
				return nil
			} else {
				return errors.Errorf("bad mode, content type, or value for file %v (%v, %v, %v). Node = %v", filePath, fileData.Mode, fileData.ContentType, fileData.Value, utils.PrettyJSON(node))
			}
			if err != nil {
				return err
			}

			err = idx.Add(&git.IndexEntry{
				Ctime: git.IndexTime{
					Seconds: int32(time.Now().Unix()),
				},
				Mtime: git.IndexTime{
					Seconds: int32(time.Now().Unix()),
				},
				Mode: git.Filemode(fileData.Mode),
				Uid:  uint32(os.Getuid()),
				Gid:  uint32(os.Getgid()),
				Size: uint32(refEntry.size),
				Id:   &refEntry.oid,
				Path: string(filePath),
			})
			return errors.Wrap(err, "error adding files to index:")
		})
		if err != nil {
			return err
		}
	}

	//
	// Write the tree and all of its children to disk
	//
	treeOid, err := idx.WriteTreeTo(repo)
	if err != nil {
		return err
	}
	tree, err := repo.LookupTree(treeOid)
	if err != nil {
		return err
	}

	//
	// Create a commit based on the new tree
	//
	var (
		author    = &git.Signature{Name: commit.Author.Name, Email: commit.Author.Email, When: commit.Author.Timestamp}
		committer = &git.Signature{Name: commit.Committer.Name, Email: commit.Committer.Email, When: commit.Committer.Timestamp}
	)

	var parentCommits []*git.Commit
	for _, pid := range commit.Parents {
		oid, err := git.NewOid(pid)
		if err != nil {
			return err
		}

		var parentCommit *git.Commit
		parentCommit, err = repo.LookupCommit(oid)
		if err != nil {
			return err
		}
		parentCommits = append(parentCommits, parentCommit)
	}

	oid, err := repo.CreateCommit("", author, committer, commit.Message, tree, parentCommits...)
	if commit.Signature != "" {
		gitCommit, err := repo.LookupCommit(oid)
		if err != nil {
			return err
		}
		oid, err = repo.CreateCommitWithSignature(gitCommit.ContentToSign(), commit.Signature, "gpgsig")
		if err != nil {
			return err
		}
	}
	return err
}

func createSubmodule(fileData FileData) (BlobEntry, error) {
	oid, err := git.NewOid(fileData.Value)
	if err != nil {
		return BlobEntry{}, errors.WithStack(err)
	}
	return BlobEntry{*oid, 0}, nil
}

func createBlob(commitHash string, filePath state.Keypath, fileData FileData, client *braidhttp.LightClient, odb *git.Odb) (BlobEntry, error) {
	blobHashStr := fileData.Value[len("blob:"):]
	absFileKeypath := state.Keypath("commits").Pushs(commitHash).Pushs("files").Push(filePath)

	var blobID blob.ID
	err := blobID.UnmarshalText([]byte(blobHashStr))
	if err != nil {
		return BlobEntry{}, err
	}

	sha1, err := git.NewOid(fileData.SHA1)
	if err != nil {
		return BlobEntry{}, err
	}
	if odb.Exists(sha1) {
		return BlobEntry{*sha1, int64(fileData.ContentLength)}, nil
	}

	have, err := client.HaveBlob(blobID)
	if err != nil {
		return BlobEntry{}, err
	} else if have {

	}

	blobReader, size, _, err := client.Get(StateURI, nil, absFileKeypath, nil, false)
	if err != nil {
		return BlobEntry{}, errors.WithStack(err)
	}
	defer blobReader.Close()
	logf("blob: %v %v (%v)", blobHashStr, filePath, utils.FileSize(size))

	blobWriter, err := odb.NewWriteStream(size, git.ObjectBlob)
	if err != nil {
		return BlobEntry{}, errors.WithStack(err)
	}

	_, err = io.Copy(blobWriter, blobReader)
	if err != nil {
		blobWriter.Close()
		return BlobEntry{}, errors.WithStack(err)
	}

	err = blobWriter.Close()
	if err != nil {
		return BlobEntry{}, errors.WithStack(err)
	}
	return BlobEntry{blobWriter.Id, size}, nil
}
