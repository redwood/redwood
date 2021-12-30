package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	git "github.com/brynbellomy/git2go"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm/braidhttp"
	"redwood.dev/tree/nelson"
	"redwood.dev/utils"
)

func fetch(client *braidhttp.LightClient, headCommitHash string) (err error) {
	defer errors.AddStack(&err)

	odb, err := repo.Odb()
	if err != nil {
		return
	}

	// Fetch just the ancestors of the requested commit
	// @@TODO: exclude anything we already have
	stack := []string{headCommitHash}
	var commits []Commit
	for len(stack) > 0 {
		commitHash := stack[0]
		stack = stack[1:]

		var stateReader io.ReadCloser
		// fmt.Println("GET", RootKeypath.Pushs("commits").Pushs(commitHash))
		stateReader, _, _, err = client.Get(StateURI, nil, RootKeypath.Pushs("commits").Pushs(commitHash), nil, true)
		if err != nil {
			return
		}
		defer stateReader.Close()

		var commit Commit
		commit.Files = state.NewMemoryNode()
		err = json.NewDecoder(stateReader).Decode(&commit)
		if err != nil {
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
		err = fetchCommit(commit.Hash, commit, client, odb, refs)
		if err != nil {
			return err
		}
	}

	return nil
}

func fetchCommit(commitHash string, commit Commit, client *braidhttp.LightClient, odb *git.Odb, refs *sync.Map) (err error) {
	defer errors.Annotate(&err, "fetchCommit")

	logf("commit: %v", commitHash)

	idx, err := git.NewIndex()
	if err != nil {
		return errors.WithStack(err)
	}

	wg := &sync.WaitGroup{}
	chErr := make(chan error)

	logf("JSON: %v", utils.PrettyJSON(commit.Files))

	err = walkLinks(commit.Files, func(linkType nelson.LinkType, linkStr string, filePath state.Keypath) error {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			defer func() {
				if err != nil {
					chErr <- err
				}
			}()

			refObj, _, _, err := client.Get(StateURI, nil, state.Keypath("commits/"+commitHash+"/files").Push(filePath).Pushs("value"), nil, true)
			if err != nil {
				err = errors.WithStack(err)
				return
			}
			defer refObj.Close()

			bs, err := ioutil.ReadAll(refObj)
			if err != nil {
				err = errors.WithStack(err)
				return
			}

			type refEntry struct {
				oid  git.Oid
				size int64
			}

			refHashStr := string(bs[len("blob:"):])
			refEntryInterface, alreadyExists := refs.LoadOrStore(refHashStr, &refEntry{git.Oid{}, 0})
			if !alreadyExists {
				absFileKeypath := state.Keypath("commits/" + commitHash + "/files").Push(filePath)

				ref, size, _, err := client.Get(StateURI, nil, absFileKeypath, nil, false)
				if err != nil {
					err = errors.WithStack(err)
					return
				}
				defer ref.Close()
				logf("blob: %v %v", linkStr, filePath)

				writeStream, err := odb.NewWriteStream(size, git.ObjectBlob)
				if err != nil {
					err = errors.WithStack(err)
					return
				}

				_, err = io.Copy(writeStream, ref)
				if err != nil {
					err = errors.WithStack(err)
					writeStream.Close()
					return
				}

				err = writeStream.Close()
				if err != nil {
					err = errors.WithStack(err)
					return
				}

				refEntry := &refEntry{writeStream.Id, size}
				refs.Store(refHashStr, refEntry)
				refEntryInterface = refEntry
			}
			entry := refEntryInterface.(*refEntry)
			oid := entry.oid
			size := entry.size

			mode, ok, err := commit.Files.FloatValue(filePath.Push(state.Keypath("mode")))
			if err != nil {
				err = errors.Wrap(err, "error fetching mode:")
				return
			} else if !ok {
				err = errors.New("missing mode")
				return
			}

			err = idx.Add(&git.IndexEntry{
				Ctime: git.IndexTime{
					Seconds: int32(time.Now().Unix()),
				},
				Mtime: git.IndexTime{
					Seconds: int32(time.Now().Unix()),
				},
				Mode: git.Filemode(int(mode)),
				Uid:  uint32(os.Getuid()),
				Gid:  uint32(os.Getgid()),
				Size: uint32(size),
				Id:   &oid,
				Path: string(filePath),
			})
			if err != nil {
				err = errors.Wrap(err, "error adding files to index:")
				return
			}
		}()
		return nil
	})
	if err != nil {
		return err
	}

	go func() {
		defer close(chErr)
		wg.Wait()
	}()
	for err := range chErr {
		return err
	}

	//
	// Write the state and all of its children to disk
	//
	stateOid, err := idx.WriteTreeTo(repo)
	if err != nil {
		return err
	}
	state, err := repo.LookupTree(stateOid)
	if err != nil {
		return err
	}

	//
	// Create a commit based on the new state
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

	_, err = repo.CreateCommit("", author, committer, commit.Message, state, parentCommits...)
	return err
}
