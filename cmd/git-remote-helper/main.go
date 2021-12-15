package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	git "github.com/brynbellomy/git2go"

	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm/braidhttp"
	"redwood.dev/tree"
	"redwood.dev/tree/nelson"
	"redwood.dev/types"
	"redwood.dev/utils"
)

// @@TODO: read keys from config
var sigkeys = func() *crypto.SigKeypair {
	sigkeys, err := crypto.GenerateSigKeypair()
	if err != nil {
		panic(err)
	}
	return sigkeys
}()

var StateURI string
var RootKeypath state.Keypath
var repo *git.Repository

type M = map[string]interface{}

func main() {
	var GIT_DIR = os.Getenv("GIT_DIR")

	if GIT_DIR == "" {
		die(errors.New("empty GIT_DIR"))
	}

	re := regexp.MustCompile(`redwood://(?:([^@]*)@)?(.*)`)
	matches := re.FindStringSubmatch(os.Args[2])
	host := matches[1]
	stateURIAndPath := strings.Split(matches[2], "/")

	StateURI = strings.Join(stateURIAndPath[:2], "/")
	RootKeypath = state.Keypath(strings.Join(stateURIAndPath[2:], string(state.KeypathSeparator)))

	gitDir, err := filepath.Abs(filepath.Dir(GIT_DIR))
	if err != nil {
		die(err)
	}

	repo, err = git.OpenRepository(gitDir)
	if err != nil {
		die(err)
	}

	client, err := braidhttp.NewLightClient("http://"+host, sigkeys, nil, false)
	if err != nil {
		die(err)
	}

	err = client.Authorize()
	if err != nil {
		die(err)
	}
	logf("redwood: authorized with " + host)

	err = speakGit(os.Stdin, os.Stdout, client)
	if err != nil {
		die(err)
	}
}

func speakGit(r io.Reader, w io.Writer, client *braidhttp.LightClient) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		text = strings.TrimSpace(text)
		logf("[git] %v", text)

		switch {

		case strings.HasPrefix(text, "capabilities"):
			fmt.Fprintln(w, "list")
			fmt.Fprintln(w, "fetch")
			fmt.Fprintln(w, "push")
			fmt.Fprintln(w)

		case strings.HasPrefix(text, "list"):
			//forPush := strings.Contains(text, "for-push")

			refs, err := getRefs(client)
			if err != nil {
				return err
			}
			for _, ref := range refs {
				fmt.Fprintln(w, ref)
			}
			fmt.Fprintln(w)

		case strings.HasPrefix(text, "fetch"):
			fetchArgs := strings.Split(text, " ")
			commitHash := fetchArgs[1]
			err := fetch(client, commitHash)
			if err != nil {
				return err
			}

			fmt.Fprintln(w)

		case strings.HasPrefix(text, "push"):
			for scanner.Scan() {
				pushSplit := strings.Split(text, " ")
				if len(pushSplit) < 2 {
					return errors.Errorf("malformed 'push' command. %q", text)
				}
				srcDstSplit := strings.Split(pushSplit[1], ":")
				if len(srcDstSplit) < 2 {
					return errors.Errorf("malformed 'push' command. %q", text)
				}
				src, dst := srcDstSplit[0], srcDstSplit[1]
				err := push(src, dst, client)
				if err != nil {
					return err
				}
				text = scanner.Text()
				if text == "" {
					break
				}
			}
			fmt.Fprintln(w)

		case text == "":
			// The blank line is the stream terminator.  We return when we see this.
			if runtime.GOOS == "windows" {
				return nil
			}

		default:
			return fmt.Errorf("unknown git speak: %v", text)
		}
	}
	return scanner.Err()
}

func getRefs(client *braidhttp.LightClient) ([]string, error) {
	stateReader, _, _, err := client.Get(StateURI, nil, RootKeypath.Push(state.Keypath("refs/heads")), nil, true)
	if errors.Cause(err) == errors.Err404 {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer stateReader.Close()

	var heads map[string]map[string]interface{}
	err = json.NewDecoder(stateReader).Decode(&heads)
	if err != nil {
		return nil, err
	}

	refsList := []string{}
	for refName, refObject := range heads {
		maybeCommitHash, exists := refObject["HEAD"]
		if !exists {
			logf("no commit for ref %v", refName)
			continue
		}
		commitHash, is := maybeCommitHash.(string)
		if !is {
			logf("no commit for ref %v", refName)
			continue
		}
		refsList = append(refsList, fmt.Sprintf("%s %s", commitHash, "refs/heads/"+refName))
	}
	refsList = append(refsList, "@refs/heads/master HEAD")
	return refsList, nil
}

type Commit struct {
	Hash      string    `json:"-"`
	Parents   []string  `json:"parents"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Author    struct {
		Name      string    `json:"name"`
		Email     string    `json:"email"`
		Timestamp time.Time `json:"timestamp"`
	} `json:"author"`
	Committer struct {
		Name      string    `json:"name"`
		Email     string    `json:"email"`
		Timestamp time.Time `json:"timestamp"`
	} `json:"committer"`
	Files *state.MemoryNode `json:"files"`
}

func fetch(client *braidhttp.LightClient, headCommitHash string) error {
	odb, err := repo.Odb()
	if err != nil {
		return errors.WithStack(err)
	}

	// Fetch just the ancestors of the requested commit
	// @@TODO: exclude anything we already have
	stack := []string{headCommitHash}
	var commits []Commit
	for len(stack) > 0 {
		commitHash := stack[0]
		stack = stack[1:]

		stateReader, _, _, err := client.Get(StateURI, nil, RootKeypath.Push(state.Keypath("commits/"+commitHash)), nil, true)
		if err != nil {
			return errors.WithStack(err)
		}
		defer stateReader.Close()

		var commit Commit
		commit.Files = state.NewMemoryNode()
		err = json.NewDecoder(stateReader).Decode(&commit)
		if err != nil {
			return err
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

			refObj, _, _, err := client.Get(StateURI, nil, state.Keypath("commits/"+commitHash+"/files").Push(filePath).Push(state.Keypath("value")), nil, true)
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
				logf("blob:    %v %v", linkStr, filePath)

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

func push(srcRefName string, destRefName string, client *braidhttp.LightClient) error {
	logf("%v -> %v", srcRefName, destRefName)

	force := strings.HasPrefix(srcRefName, "+")
	if force {
		srcRefName = srcRefName[1:]
	}

	ref, err := repo.References.Dwim(srcRefName)
	if err != nil {
		return errors.WithStack(err)
	}
	headCommitID := ref.Target()

	stack := []*git.Oid{headCommitID}
	for len(stack) > 0 {
		commitID := stack[0]
		stack = stack[1:]

		txID := state.VersionFromBytes(commitID[:])
		_, err := client.FetchTx(StateURI, txID)
		if err == errors.Err404 {
			err = pushCommit(commitID, destRefName, client)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		commit, err := repo.LookupCommit(commitID)
		if err != nil {
			return err
		}

		parentCount := commit.ParentCount()
		for i := uint(0); i < parentCount; i++ {
			stack = append(stack, commit.ParentId(i))
		}
	}

	return pushRef(destRefName, headCommitID, client)
}

func pushRef(destRefName string, commitID *git.Oid, client *braidhttp.LightClient) (err error) {
	defer errors.AddStack(&err)

	branchKeypath := RootKeypath.Push(state.Keypath(destRefName))

	_, _, _, err = client.Get(StateURI, nil, nil, nil, true)
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
							// "*": M{
							// 	"^\\.refs\\..*": M{
							// 		"write": true,
							// 	},
							// 	"^\\.commits\\.[a-f0-9]+\\.parents": M{
							// 		"write": true,
							// 	},
							// 	"^\\.commits\\.[a-f0-9]+\\.message": M{
							// 		"write": true,
							// 	},
							// 	"^\\.commits\\.[a-f0-9]+\\.timestamp": M{
							// 		"write": true,
							// 	},
							// 	"^\\.commits\\.[a-f0-9]+\\.author": M{
							// 		"write": true,
							// 	},
							// 	"^\\.commits\\.[a-f0-9]+\\.committer": M{
							// 		"write": true,
							// 	},
							// 	"^\\.commits\\.[a-f0-9]+\\.files": M{
							// 		"write": true,
							// 	},
							// },
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

	} else if err != nil {
		return err
	}

	parentID, err := state.VersionFromHex(commitID.String())
	if err != nil {
		return err
	}

	tx := tree.Tx{
		ID:       state.RandomVersion(),
		StateURI: StateURI,
		From:     sigkeys.Address(),
		Parents:  []state.Version{parentID},
		Patches: []tree.Patch{{
			Keypath:   branchKeypath.Push(state.Keypath("HEAD")),
			ValueJSON: mustJSON(commitID.String()),
		}, {
			// 	Keypath: branchKeypath.Push(state.Keypath("reflog")),
			// 	Range:   &state.Range{0, 0},
			// 	ValueJSON: mustJSON(    commitID.String()),
			// }, {
			Keypath: branchKeypath.Push(state.Keypath("worktree")),
			ValueJSON: mustJSON(map[string]interface{}{
				"Content-Type": "link",
				"value":        "state:" + StateURI + "/commits/" + commitID.String() + "/files",
			}),
		}},
	}

	return client.Put(context.Background(), tx)
}

func mustJSON(x interface{}) []byte {
	bs, err := json.Marshal(x)
	if err != nil {
		panic(err)
	}
	return bs
}

func pushCommit(commitID *git.Oid, destRefName string, client *braidhttp.LightClient) error {
	commit, err := repo.LookupCommit(commitID)
	if err != nil {
		return errors.WithStack(err)
	}
	defer commit.Free()

	commitTree, err := commit.Tree()
	if err != nil {
		return errors.WithStack(err)
	}
	defer commitTree.Free()

	type uploaded struct {
		name        string
		contentType string
		gitOid      string
		hash        types.Hash
		mode        git.Filemode
	}
	type stateEntry struct {
		oid        *git.Oid
		objectType git.ObjectType
		fullPath   string
		mode       git.Filemode
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	chEntries := make(chan stateEntry)
	chUploaded := make(chan uploaded)
	chErr := make(chan error)
	var wg sync.WaitGroup

	go func() {
		defer close(chEntries)
		err = commitTree.Walk(func(rootPath string, entry *git.TreeEntry) int {
			select {
			case <-ctx.Done():
				return -1
			case chEntries <- stateEntry{entry.Id, entry.Type, filepath.Join(rootPath, entry.Name), entry.Filemode}:
			}
			return 0
		})
	}()

	for stateEntry := range chEntries {
		if stateEntry.objectType != git.ObjectBlob {
			continue
		}

		blob, err := repo.LookupBlob(stateEntry.oid)
		if err != nil {
			logf("error: %v", err)
			return errors.WithStack(err)
		}

		stateEntry := stateEntry
		wg.Add(1)
		go func() {
			defer wg.Done()

			data := blob.Contents()
			contentType, err := utils.SniffContentType(stateEntry.fullPath, bytes.NewBuffer(data))
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case chErr <- err:
					logf("error: %v", err)
					return
				}
			}

			resp, err := client.StoreBlob(bytes.NewReader(blob.Contents()))
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case chErr <- err:
					logf("error: %v", err)
					return
				}
			}
			select {
			case <-ctx.Done():
				return
			case chUploaded <- uploaded{stateEntry.fullPath, contentType, stateEntry.oid.String(), resp.SHA1, stateEntry.mode}:
			}
		}()
	}

	go func() {
		defer close(chUploaded)
		wg.Wait()
	}()

	go func() {
		defer close(chErr)
		var err error
		defer func() {
			if err != nil {
				select {
				case chErr <- err:
					logf("error: %v", err)
					cancel()
				case <-ctx.Done():
					return
				}
			}
		}()

		var parents []state.Version
		var parentStrs []string
		if commit.ParentCount() == 0 {
			parents = []state.Version{tree.GenesisTxID}
		} else {
			for i := uint(0); i < commit.ParentCount(); i++ {
				parentStr := commit.ParentId(i).String()
				parentStrs = append(parentStrs, parentStr)

				var parentID state.Version
				var err error
				parentID, err = state.VersionFromHex(parentStr)
				if err != nil {
					return
				}
				parents = append(parents, parentID)
			}
		}

		commitHash := commit.Id().String()
		commitID, err := state.VersionFromHex(commitHash)
		if err != nil {
			return
		}

		tx := tree.Tx{
			ID:         commitID,
			StateURI:   StateURI,
			From:       sigkeys.Address(),
			Parents:    parents,
			Checkpoint: true,
		}

		tx.Patches = append(tx.Patches, tree.Patch{
			Keypath:   state.Keypath("commits/" + commitHash + "/parents"),
			ValueJSON: mustJSON(parentStrs),
		})

		tx.Patches = append(tx.Patches, tree.Patch{
			Keypath:   state.Keypath("commits/" + commitHash + "/message"),
			ValueJSON: mustJSON(commit.Message()),
		})
		timeStr, err := commit.Time().MarshalText()
		if err != nil {
			return
		}
		tx.Patches = append(tx.Patches, tree.Patch{
			Keypath:   state.Keypath("commits/" + commitHash + "/timestamp"),
			ValueJSON: mustJSON(string(timeStr)),
		})
		author := commit.Author()
		authorTimeStr, err := author.When.MarshalText()
		if err != nil {
			return
		}
		tx.Patches = append(tx.Patches, tree.Patch{
			Keypath: state.Keypath("commits/" + commitHash + "/author"),
			ValueJSON: mustJSON(M{
				"name":      author.Name,
				"email":     author.Email,
				"timestamp": string(authorTimeStr),
			}),
		})
		committer := commit.Committer()
		committerTimeStr, err := committer.When.MarshalText()
		if err != nil {
			return
		}
		tx.Patches = append(tx.Patches, tree.Patch{
			Keypath: state.Keypath("commits/" + commitHash + "/committer"),
			ValueJSON: mustJSON(M{
				"name":      committer.Name,
				"email":     committer.Email,
				"timestamp": string(committerTimeStr),
			}),
		})

		fileTree := make(map[string]interface{})
		for uploaded := range chUploaded {
			select {
			case <-ctx.Done():
				return
			default:
			}
			setValueAtKeypath(fileTree, strings.Split(uploaded.name, "/"), M{
				"Content-Type": "link",
				"value":        "blob:sha1:" + uploaded.hash.Hex(),
				"mode":         int(uploaded.mode),
			}, true)
		}
		tx.Patches = append(tx.Patches, tree.Patch{
			Keypath:   state.Keypath("commits/" + commitHash + "/files"),
			ValueJSON: mustJSON(fileTree),
		})

		tx.Patches = append(tx.Patches, tree.Patch{
			Keypath: state.Keypath("refs/heads/master"),
			ValueJSON: mustJSON(M{
				"HEAD": commitHash,
				"worktree": map[string]interface{}{
					"Content-Type": "link",
					"value":        "state:somegitprovider.org/gitdemo/commits/" + commitHash + "/files",
				},
			}),
		})

		err = client.Put(context.Background(), tx)
	}()

	select {
	case err, noErr := <-chErr:
		if !noErr {
			cancel()
			return err
		}
	}

	return nil
}

func die(err error) {
	logf("error: %+v", err)
	os.Exit(1)
}

func logf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

func walkLinks(n state.Node, fn func(linkType nelson.LinkType, linkStr string, keypath state.Keypath) error) error {
	iter := n.DepthFirstIterator(nil, false, 0)
	defer iter.Close()

	for iter.Rewind(); iter.Valid(); iter.Next() {
		node := iter.Node()

		parentKeypath, key := node.Keypath().Pop()
		if key.Equals(nelson.ContentTypeKey) {
			parentNode := n.NodeAt(parentKeypath, nil)

			contentType, err := nelson.GetContentType(parentNode)
			if err != nil && errors.Cause(err) != errors.Err404 {
				return err
			} else if contentType != "link" {
				continue
			}

			linkStr, _, err := parentNode.StringValue(nelson.ValueKey)
			if err != nil {
				return err
			}
			linkType, linkValue := nelson.DetermineLinkType(linkStr)

			err = fn(linkType, linkValue, parentKeypath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func setValueAtKeypath(x interface{}, keypath []string, val interface{}, clobber bool) {
	if len(keypath) == 0 {
		panic("bad")
	}

	var cur interface{} = x
	for i := 0; i < len(keypath)-1; i++ {
		key := keypath[i]

		if asMap, isMap := cur.(map[string]interface{}); isMap {
			var exists bool
			cur, exists = asMap[key]
			if !exists {
				if !clobber {
					return
				}
				asMap[key] = make(map[string]interface{})
				cur = asMap[key]
			}

		} else if asSlice, isSlice := cur.([]interface{}); isSlice {
			i, err := strconv.Atoi(key)
			if err != nil {
				panic(err)
			}
			cur = asSlice[i]
		} else {
			panic("bad")
		}
	}
	if asMap, isMap := cur.(map[string]interface{}); isMap {
		asMap[keypath[len(keypath)-1]] = val
	} else {
		panic("bad")
	}
}
