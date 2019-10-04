package gitresolver

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/libgit2/git2go"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/brynbellomy/redwood"
)

type GitResolver struct {
	repoRoot string
	branch   string
}

func (s *GitResolver) ResolveState(state interface{}, patch redwood.Patch) (newState interface{}, err error) {
	repo, err := git.OpenRepository(s.repoRoot)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	//
	// Create a blank index.  If a parent commit was specified, copy it into the index as our
	// initial index state.
	//
	idx, err := git.NewIndex()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var parentCommit *git.Commit

	branch, err := repo.LookupBranch(s.branch, git.BranchLocal)
	if git.IsErrorCode(err, git.ErrNotFound) == false {
		branchHeadObj, err := branch.Peel(git.ObjectCommit)
		if err != nil {
			return nil, err
		}

		parentCommit, err = branchHeadObj.AsCommit()
		if err != nil {
			return nil, err
		}
	}

	//
	// Stream the updates into the new index and the ODB
	//
	odb, err := repo.Odb()
	if err != nil {
		return nil, err
	}

	err = walkLeaves(state, func(keypath []string, val interface{}) error {
		filepath := strings.Join(keypath, "/")
		ctime := time.Now().UTC().Unix()
		mtime := ctime

		var filedata []byte
		switch v := val.(type) {
		case string:
			filedata = []byte(v)
		default:
			var err error
			filedata, err = json.Marshal(val)
			if err != nil {
				return err
			}
		}

		uncompressedSize := len(filedata)

		writeStream, err := odb.NewWriteStream(int64(uncompressedSize), git.ObjectBlob)
		if err != nil {
			return err
		}
		defer func() {
			if writeStream != nil {
				writeStream.Close()
			}
		}()

		n, err := writeStream.Write(filedata)
		if err != nil {
			return err
		} else if n < len(filedata) {
			return errors.New("[noderpc] CreateCommit i/o error: did not finish writing")
		}

		err = writeStream.Close()
		if err != nil {
			return err
		}

		oid := writeStream.Id
		writeStream = nil

		// Adding a new entry
		idx.Add(&git.IndexEntry{
			Ctime: git.IndexTime{
				Seconds: int32(ctime),
			},
			Mtime: git.IndexTime{
				Seconds: int32(mtime),
			},
			Mode: git.FilemodeBlob,
			Uid:  uint32(os.Getuid()),
			Gid:  uint32(os.Getgid()),
			Size: uint32(uncompressedSize),
			Id:   &oid,
			Path: filepath,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	//
	// Write the new tree object to disk
	//
	treeOid, err := idx.WriteTreeTo(repo)
	if err != nil {
		return nil, err
	}

	tree, err := repo.LookupTree(treeOid)
	if err != nil {
		return nil, err
	}

	//
	// Create a commit based on the new tree
	//
	var (
		now       = time.Now()
		message   = "new commit"
		author    = &git.Signature{Name: "bryn", Email: "bryn.bellomy@gmail.com", When: now}
		committer = &git.Signature{Name: "bryn", Email: "bryn.bellomy@gmail.com", When: now}
	)

	var parentCommits []*git.Commit
	if parentCommit != nil {
		parentCommits = append(parentCommits, parentCommit)
	}

	newCommitHash, err := repo.CreateCommit("refs/heads/"+s.branch, author, committer, message, tree, parentCommits...)
	if err != nil {
		return nil, err
	}

	log.Infof("[git] created new commit %v", newCommitHash.String())

	return state, nil
}

func walkLeaves(tree interface{}, fn func(keypath []string, val interface{}) error) error {
	type item struct {
		val     interface{}
		keypath []string
	}

	stack := []item{{val: tree, keypath: []string{}}}
	var current item

	for len(stack) > 0 {
		current = stack[0]
		stack = stack[1:]

		asMap, isMap := current.val.(map[string]interface{})
		if !isMap {
			err := fn(current.keypath, current.val)
			if err != nil {
				return err
			}
		} else {
			for key := range asMap {
				stack = append(stack, item{
					val:     asMap[key],
					keypath: append(current.keypath, key),
				})
			}
		}
	}
	return nil
}
