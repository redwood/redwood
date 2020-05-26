package redwood

import (
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"sort"
	"time"

	"github.com/brynbellomy/git2go"
	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/nelson"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

var log = ctx.NewLogger("hi")

type gitResolver struct{}

func NewGitResolver(config tree.Node, internalState map[string]interface{}) (_ Resolver, err error) {
	return &gitResolver{}, nil
}

func (r *gitResolver) InternalState() map[string]interface{} {
	return make(map[string]interface{})
}

func (r *gitResolver) ResolveState(
	state tree.Node,
	refStore RefStore,
	sender types.Address,
	txID types.ID,
	parents []types.ID,
	patches []Patch,
) (err error) {
	defer withStack(&err)

	node := tree.NewMemoryNode()
	for _, patch := range patches {
		err := node.Set(patch.Keypath, patch.Range, patch.Val)
		if err != nil {
			return err
		}
	}

	refSubkeys := node.NodeAt(tree.Keypath("refs").Pushs("heads"), nil).Subkeys()
	refName := string(refSubkeys[0])

	commitSubkeys := node.NodeAt(tree.Keypath("commits"), nil).Subkeys()
	commitHash := commitSubkeys[0]
	commitNode := node.NodeAt(tree.Keypath("commits").Push(commitHash), nil)

	//
	// Stream the updates into the new index and the ODB
	//
	repo, err := git.OpenRepository("/tmp/myrepo/.git")
	if err != nil {
		return errors.WithStack(err)
	}

	odb, err := repo.Odb()
	if err != nil {
		return err
	}

	thisCommit, err := r.createCommitFromNode(commitNode, refName, refStore, repo, odb)
	if err != nil {
		return err
	}

	//
	// Perform an auto-merge, if applicable
	//
	mergeCommit, err := r.maybeAutoMerge(state, thisCommit, refName, refStore, repo, odb)
	if err != nil {
		return err
	}

	var refOID *git.Oid
	if mergeCommit != nil {
		refOID = mergeCommit.Id()
	} else {
		refOID = thisCommit.Id()
	}

	refNameRemote := "refs/remotes/origin/" + refName
	ref, err := repo.References.Lookup(refNameRemote)
	if err != nil {
		ref, err = repo.References.Create(refNameRemote, refOID, false, "")
		if err != nil {
			return err
		}
	}
	_, err = ref.SetTarget(refOID, "")
	if err != nil {
		return err
	}

	//
	// Apply the commit portion of the patch to the state tree
	//
	for _, patch := range patches {
		// Use the commit hash we received from `repo.CreateCommit`
		if patch.Keypath.Part(0).Equals(tree.Keypath("commits")) {
			keypath := patch.Keypath.Part(0).Pushs(thisCommit.Id().String())
			err := state.Set(keypath, patch.Range, patch.Val)
			if err != nil {
				return err
			}
		}
	}

	// Set the ref
	return state.Set(tree.Keypath("refs").Pushs("heads").Pushs(refName), nil, refOID.String())
}

func (r *gitResolver) maybeAutoMerge(
	state tree.Node,
	thisCommit *git.Commit,
	refName string,
	refStore RefStore,
	repo *git.Repository,
	odb *git.Odb,
) (_ *git.Commit, err error) {
	defer withStack(&err)

	commitsWithSharedParent := make(map[git.Oid]struct{})

	thisCommitParents := make(map[git.Oid]struct{}, thisCommit.ParentCount())
	for i := uint(0); i < thisCommit.ParentCount(); i++ {
		parentOID := thisCommit.ParentId(i)
		thisCommitParents[*parentOID] = struct{}{}
	}

	// @@TODO: use an index to make this efficient
	err = func() error {
		commitsIter := state.ChildIterator(tree.Keypath("commits"), true, 10)
		defer commitsIter.Close()

		for commitsIter.Rewind(); commitsIter.Valid(); commitsIter.Next() {
			otherCommitNode := commitsIter.Node()

			nodeType, _, numParents, err := otherCommitNode.NodeInfo(tree.Keypath("parents"))
			if errors.Cause(err) == types.Err404 {
				continue
			} else if err != nil {
				return err
			} else if nodeType != tree.NodeTypeSlice {
				continue
			}

			for i := uint64(0); i < numParents; i++ {
				parentStr, exists, err := otherCommitNode.StringValue(tree.Keypath("parents").PushIndex(i))
				if err != nil {
					continue
				} else if !exists {
					continue
				}

				parentOid, err := git.NewOid(parentStr)
				if err != nil {
					continue
				}

				if _, exists := thisCommitParents[*parentOid]; exists {
					shaStr := string(otherCommitNode.Keypath().Part(-1))
					shaOID, err := git.NewOid(shaStr)
					if err != nil {
						continue
					}
					commitsWithSharedParent[*shaOID] = struct{}{}
				}
			}
		}
		return nil
	}()
	if err != nil {
		return nil, err
	} else if len(commitsWithSharedParent) == 0 {
		return nil, nil
	}

	commitsWithSharedParentSlice := make([]git.Oid, len(commitsWithSharedParent)+1)
	i := 0
	for oid := range commitsWithSharedParent {
		commitsWithSharedParentSlice[i] = oid
		i++
	}
	commitsWithSharedParentSlice[len(commitsWithSharedParentSlice)-1] = *thisCommit.Id()

	var cs []string
	for _, c := range commitsWithSharedParentSlice {
		cs = append(cs, c.String())
	}

	// Arbitrary (but deterministic) merge resolution: convert the commit SHAs to integers
	// and order them numerically.
	sort.Slice(commitsWithSharedParentSlice, func(i, j int) bool {
		x := big.NewInt(0).SetBytes(commitsWithSharedParentSlice[i][:])
		y := big.NewInt(0).SetBytes(commitsWithSharedParentSlice[j][:])
		return x.Cmp(y) < 0
	})

	mergeCommitNode := tree.NewMemoryNode()

	for _, commitSHA := range commitsWithSharedParentSlice {
		// log.Warn(commitSHA.String())
		err := func() error {
			filesKeypath := tree.Keypath("commits").
				Pushs(commitSHA.String()).
				Pushs("files")

			filesIter := state.ChildIterator(filesKeypath, true, 10)
			defer filesIter.Close()

			for filesIter.Rewind(); filesIter.Valid(); filesIter.Next() {
				fileNode := filesIter.Node()
				relKeypath := fileNode.Keypath().RelativeTo(filesKeypath)

				fileNode, err := fileNode.CopyToMemory(nil, nil)
				if err != nil {
					return err
				}

				err = mergeCommitNode.Set(tree.Keypath("files").Push(relKeypath), nil, fileNode)
				if err != nil {
					return err
				}
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}

	mergeCommitParents := make([]interface{}, len(commitsWithSharedParentSlice))
	for i := 0; i < len(commitsWithSharedParentSlice); i++ {
		mergeCommitParents[i] = commitsWithSharedParentSlice[i].String()
	}

	mergeCommitNode.Set(tree.Keypath("message"), nil, "Redwood auto-merge")
	mergeCommitNode.Set(tree.Keypath("parents"), nil, mergeCommitParents)
	mergeCommitNode.Set(tree.Keypath("author").Pushs("name"), nil, "Redwood")
	mergeCommitNode.Set(tree.Keypath("author").Pushs("email"), nil, "redwood@localhost")
	mergeCommitNode.Set(tree.Keypath("author").Pushs("timestamp"), nil, time.Time{}.Format(time.RFC3339))
	mergeCommitNode.Set(tree.Keypath("committer").Pushs("name"), nil, "Redwood")
	mergeCommitNode.Set(tree.Keypath("committer").Pushs("email"), nil, "redwood@localhost")
	mergeCommitNode.Set(tree.Keypath("committer").Pushs("timestamp"), nil, time.Time{}.Format(time.RFC3339))

	mergeCommit, err := r.createCommitFromNode(mergeCommitNode, refName, refStore, repo, odb)
	if err != nil {
		return nil, err
	}

	err = state.Set(tree.Keypath("commits").Push(tree.Keypath(mergeCommit.Id().String())), nil, mergeCommitNode)
	if err != nil {
		return nil, err
	}

	return mergeCommit, nil
}

func (r *gitResolver) createCommitFromNode(commitNode tree.Node, refName string, refStore RefStore, repo *git.Repository, odb *git.Odb) (_ *git.Commit, err error) {
	defer withStack(&err)

	numParents, err := commitNode.NodeAt(tree.Keypath("parents"), nil).Length()
	if err != nil {
		return nil, err
	}

	var parentOIDs []*git.Oid
	for i := uint64(0); i < numParents; i++ {
		parentOIDStr, exists, err := commitNode.StringValue(tree.Keypath("parents").PushIndex(i))
		if err != nil {
			return nil, err
		} else if !exists {
			return nil, errors.New("bad commit parent")
		}
		oid, err := git.NewOid(parentOIDStr)
		if err != nil {
			return nil, err
		}
		parentOIDs = append(parentOIDs, oid)
	}

	message, exists, err := commitNode.StringValue(tree.Keypath("message"))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.New("commit is missing 'message' field")
	}
	authorName, exists, err := commitNode.StringValue(tree.Keypath("author").Pushs("name"))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.New("commit is missing 'author/name' field")
	}
	authorEmail, exists, err := commitNode.StringValue(tree.Keypath("author").Pushs("email"))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.New("commit is missing 'author/email' field")
	}
	authorTimestampStr, exists, err := commitNode.StringValue(tree.Keypath("author").Pushs("timestamp"))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.New("commit is missing 'author/timestamp' field")
	}
	committerName, exists, err := commitNode.StringValue(tree.Keypath("committer").Pushs("name"))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.New("commit is missing 'committer/name' field")
	}
	committerEmail, exists, err := commitNode.StringValue(tree.Keypath("committer").Pushs("email"))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.New("commit is missing 'committer/email' field")
	}
	committerTimestampStr, exists, err := commitNode.StringValue(tree.Keypath("committer").Pushs("timestamp"))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.New("commit is missing 'committer/timestamp' field")
	}

	authorTimestamp, err := time.Parse(time.RFC3339, authorTimestampStr)
	if err != nil {
		return nil, err
	}
	committerTimestamp, err := time.Parse(time.RFC3339, committerTimestampStr)
	if err != nil {
		return nil, err
	}

	var parentCommits []*git.Commit
	for _, parentOID := range parentOIDs {
		commit, err := repo.LookupCommit(parentOID)
		if err != nil {
			return nil, err
		}
		parentCommits = append(parentCommits, commit)
	}

	idx, err := git.NewIndex()
	if err != nil {
		return nil, err
	}

	iter := commitNode.Iterator(tree.Keypath("files"), true, 10)
	defer iter.Close()

	baseKeypath := commitNode.Keypath().Pushs("files")

	for iter.Rewind(); iter.Valid(); iter.Next() {
		fileNode := iter.Node()

		_, valueType, _, err := fileNode.NodeInfo(nelson.ValueKey)
		if errors.Cause(err) == types.Err404 {
			continue
		} else if err != nil {
			return nil, err
		}

		contentType, _, err := fileNode.StringValue(nelson.ContentTypeKey)
		if err != nil {
			return nil, err
		}

		if contentType == "link" {
			linkStr, _, err := fileNode.StringValue(nelson.ValueKey)
			if err != nil {
				return nil, err
			}
			linkType, linkValue := nelson.DetermineLinkType(linkStr)
			if linkType != nelson.LinkTypeRef {
				// @@TODO: might want to support state links?
				continue
			}

			var refID types.RefID
			err = refID.UnmarshalText([]byte(linkValue))
			if err != nil {
				return nil, err
			} else if refID.HashAlg != types.SHA1 {
				// @@TODO: warn?  use refstore to find it?
				continue
			}

			objectReader, objectSize, err := refStore.Object(refID)
			if err != nil {
				return nil, err
			}

			filename := string(fileNode.Keypath().RelativeTo(baseKeypath))

			blobOID, err := r.addBlobToODB(objectReader, objectSize, filename, odb, idx)
			if err != nil {
				return nil, err
			}

			err = idx.Add(&git.IndexEntry{
				Ctime: git.IndexTime{
					// Seconds: int32(ctime),
				},
				Mtime: git.IndexTime{
					// Seconds: int32(mtime),
				},
				Mode: git.FilemodeBlob,
				Uid:  uint32(os.Getuid()),
				Gid:  uint32(os.Getgid()),
				Size: uint32(objectSize),
				Id:   blobOID,
				Path: filename,
			})
			if err != nil {
				return nil, err
			}

		} else if valueType == tree.ValueTypeString {
			// @@TODO: handle inline files
			continue
		}
	}

	//
	// Write the new tree object to disk
	//
	treeOid, err := idx.WriteTreeTo(repo)
	if err != nil {
		return nil, err
	}

	commitTree, err := repo.LookupTree(treeOid)
	if err != nil {
		return nil, err
	}

	//
	// Create a commit based on the new tree
	//
	var (
		author    = &git.Signature{Name: authorName, Email: authorEmail, When: authorTimestamp}
		committer = &git.Signature{Name: committerName, Email: committerEmail, When: committerTimestamp}
	)
	var parentCommitStrs []string
	for _, parent := range parentCommits {
		parentCommitStrs = append(parentCommitStrs, parent.Id().String())
	}

	// @@TODO: support other remotes
	commitOID, err := repo.CreateCommit("", author, committer, message, commitTree, parentCommits...)
	if err != nil {
		return nil, err
	}

	return repo.LookupCommit(commitOID)
}

func (r *gitResolver) addBlobToODB(objectReader io.Reader, objectSize int64, filename string, odb *git.Odb, idx *git.Index) (*git.Oid, error) {
	writeStream, err := odb.NewWriteStream(objectSize, git.ObjectBlob)
	if err != nil {
		return nil, err
	}
	defer func() {
		if writeStream != nil {
			writeStream.Close()
		}
	}()

	objectData, err := ioutil.ReadAll(objectReader)
	if err != nil {
		return nil, err
	}

	n, err := writeStream.Write(objectData)
	if err != nil {
		return nil, err
	} else if n < len(objectData) {
		return nil, errors.New("did not finish writing commit")
	}

	err = writeStream.Close()
	if err != nil {
		return nil, err
	}
	oid := writeStream.Id
	writeStream = nil // necessary to prevent the defer statement above from panicking
	return &oid, nil
}
