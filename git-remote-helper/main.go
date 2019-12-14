package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brynbellomy/git2go"
	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood"
)

// @@TODO: read keys from config
var sigkeys = func() *redwood.SigningKeypair {
	sigkeys, err := redwood.GenerateSigningKeypair()
	if err != nil {
		panic(err)
	}
	return sigkeys
}()

var StateURI string
var RootKeypath []string
var repo *git.Repository

type M = map[string]interface{}

func main() {
	var GIT_DIR = os.Getenv("GIT_DIR")

	if GIT_DIR == "" {
		die(errors.New("empty GIT_DIR"))
	}

	remoteURLParts := strings.Split(strings.Replace(os.Args[2], "redwood://", "", -1), "/")
	StateURI = strings.Join(remoteURLParts[:2], "/")
	RootKeypath = remoteURLParts[2:]

	gitDir, err := filepath.Abs(filepath.Dir(GIT_DIR))
	if err != nil {
		die(err)
	}

	repo, err = git.OpenRepository(gitDir)
	if err != nil {
		die(err)
	}

	// @@TODO: read node url from config
	client, err := redwood.NewHTTPClient("https://localhost:21232", sigkeys)
	if err != nil {
		die(err)
	}

	err = client.Authorize()
	if err != nil {
		die(err)
	}

	err = speakGit(os.Stdin, os.Stdout, client)
	if err != nil {
		die(err)
	}
}

func speakGit(r io.Reader, w io.Writer, client redwood.HTTPClient) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		text = strings.TrimSpace(text)
		fmt.Fprintln(os.Stderr, "[git]", text)

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

func getRefs(client redwood.HTTPClient) ([]string, error) {
	stateReader, _, _, err := client.Get(StateURI+"-reflog", nil, append(RootKeypath, "refs", "heads"), true)
	if err != nil {
		return nil, err
	}
	defer stateReader.Close()

	var heads map[string]interface{}
	err = json.NewDecoder(stateReader).Decode(&heads)
	if err != nil {
		return nil, err
	}

	refsList := []string{}
	for refName, commitHash := range heads {
		commitHashStr, is := commitHash.(string)
		if !is {
			logf("no commit for ref %v", refName)
			continue
		}
		refsList = append(refsList, fmt.Sprintf("%s %s", commitHashStr, "refs/heads/"+refName))
	}
	refsList = append(refsList, "@refs/heads/master HEAD")
	return refsList, nil
}

func fetch(client redwood.HTTPClient, commitHash string) error {
	odb, err := repo.Odb()
	if err != nil {
		return errors.WithStack(err)
	}

	idx, err := git.NewIndex()
	if err != nil {
		return errors.WithStack(err)
	}

	// Fetch just the ancestors of the requested commit
	// @@TODO: exclude anything we already have
	txID, err := redwood.IDFromHex(commitHash)
	if err != nil {
		return errors.WithStack(err)
	}
	txs := map[redwood.ID]*redwood.Tx{}
	stack := []redwood.ID{txID}
	branch := []redwood.ID{txID}
	for len(stack) > 0 {
		txID := stack[0]
		stack = stack[1:]

		tx, err := client.FetchTx(StateURI, txID)
		if err != nil {
			return errors.WithStack(err)
		}

		txs[txID] = tx

		for _, pid := range tx.Parents {
			if pid == redwood.GenesisTxID {
				continue
			}
			stack = append(stack, pid)
			branch = append(branch, pid)
		}
	}

	var wg sync.WaitGroup
	chErr := make(chan error)

	ctrl, err := redwood.NewController(redwood.Address{}, StateURI, fetch_onTxProcessed(&wg, chErr, client, odb, idx), nil, nil)
	if err != nil {
		return err
	}
	err = ctrl.Start()
	if err != nil {
		return err
	}

	wg.Add(len(branch))
	for i := len(branch) - 1; i >= 0; i-- {
		txID := branch[i]
		ctrl.AddTx(txs[txID])
	}

	go func() {
		defer close(chErr)
		wg.Wait()
	}()

	for err := range chErr {
		logf("error: %v", errors.WithStack(err))
	}

	return nil
}

func fetch_onTxProcessed(wg *sync.WaitGroup, chErr chan error, client redwood.HTTPClient, odb *git.Odb, idx *git.Index) redwood.TxProcessedHandler {
	var done bool
	return func(tx *redwood.Tx, state interface{}) {
		defer wg.Done()
		if done {
			return
		}

		var err error
		defer func() {
			if err != nil {
				done = true
				chErr <- err
			}
		}()

		commitFiles, exists := getMap(state, append(RootKeypath, "files"))
		if !exists {
			err = errors.New("missing commit files")
			return
		}
		commitTimestampStr, exists := getString(state, append(RootKeypath, "timestamp"))
		if !exists {
			err = errors.New("missing commit timestamp")
			return
		}
		var commitTimestamp time.Time
		err = commitTimestamp.UnmarshalText([]byte(commitTimestampStr))
		if err != nil {
			return
		}
		commitMessage, exists := getString(state, append(RootKeypath, "message"))
		if !exists {
			err = errors.New("missing commit message")
			return
		}
		commitAuthorName, exists := getString(state, append(RootKeypath, "author", "name"))
		if !exists {
			err = errors.New("missing commit author name")
			return
		}
		commitAuthorEmail, exists := getString(state, append(RootKeypath, "author", "email"))
		if !exists {
			err = errors.New("missing commit author email")
			return
		}
		commitAuthorTimestampStr, exists := getString(state, append(RootKeypath, "author", "timestamp"))
		if !exists {
			err = errors.New("missing commit author timestamp")
			return
		}
		var commitAuthorTimestamp time.Time
		err = commitAuthorTimestamp.UnmarshalText([]byte(commitAuthorTimestampStr))
		if err != nil {
			return
		}
		commitCommitterName, exists := getString(state, append(RootKeypath, "committer", "name"))
		if !exists {
			err = errors.New("missing commit commiter name")
			return
		}
		commitCommitterEmail, exists := getString(state, append(RootKeypath, "committer", "email"))
		if !exists {
			err = errors.New("missing commit committer email")
			return
		}
		commitCommitterTimestampStr, exists := getString(state, append(RootKeypath, "committer", "timestamp"))
		if !exists {
			err = errors.New("missing commit committer timestamp")
			return
		}
		var commitCommitterTimestamp time.Time
		err = commitCommitterTimestamp.UnmarshalText([]byte(commitCommitterTimestampStr))
		if err != nil {
			return
		}

		err = redwood.WalkLinks(commitFiles, func(linkType redwood.LinkType, linkStr string, keypath []string, val map[string]interface{}) error {
			ref, size, _, err := client.Get(StateURI, &tx.ID, append([]string{"files"}, keypath...), false)
			if err != nil {
				return err
			}
			defer ref.Close()

			writeStream, err := odb.NewWriteStream(size, git.ObjectBlob)
			if err != nil {
				return errors.WithStack(err)
			}

			_, err = io.Copy(writeStream, ref)
			if err != nil {
				writeStream.Close()
				return errors.WithStack(err)
			}

			err = writeStream.Close()
			if err != nil {
				return errors.WithStack(err)
			}

			refJSONReader, _, _, err := client.Get(StateURI, &tx.ID, append([]string{"files"}, keypath...), true)
			if err != nil {
				return err
			}
			defer refJSONReader.Close()
			refJSON := make(map[string]interface{})
			err = json.NewDecoder(refJSONReader).Decode(&refJSON)
			if err != nil {
				return err
			}

			oid := writeStream.Id
			filename := strings.Join(keypath, "/")
			mode, exists := getFloat64(refJSON, []string{"mode"})
			if !exists {
				return errors.New("missing mode")
			}

			_, err = idx.EntryByPath(filename, int(git.IndexStageNormal))
			if err != nil && git.IsErrorCode(err, git.ErrNotFound) == false {
				return errors.WithStack(err)

			} else if git.IsErrorCode(err, git.ErrNotFound) {
				// Adding a new entry
				idx.Add(&git.IndexEntry{
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
					Path: filename,
				})

			}
			//else {
			//    // Updating an existing entry
			//    entry.Id = &oid
			//    entry.Size = uint32(size)
			//    entry.Ctime.Seconds = int32(ctime)
			//    entry.Mtime.Seconds = int32(mtime)
			//}
			return nil
		})
		if err != nil {
			return
		}

		//
		// Write the tree and all of its children to disk
		//
		treeOid, err := idx.WriteTreeTo(repo)
		if err != nil {
			return
		}
		tree, err := repo.LookupTree(treeOid)
		if err != nil {
			return
		}

		//
		// Create a commit based on the new tree
		//
		var (
			message   = commitMessage
			author    = &git.Signature{Name: commitAuthorName, Email: commitAuthorEmail, When: commitAuthorTimestamp}
			committer = &git.Signature{Name: commitCommitterName, Email: commitCommitterEmail, When: commitCommitterTimestamp}
		)

		var parentCommits []*git.Commit
		if tx.Parents[0] != redwood.GenesisTxID {
			for _, p := range tx.Parents {
				oid := git.NewOidFromBytes(p[:])
				if err != nil {
					return
				}

				var parentCommit *git.Commit
				parentCommit, err = repo.LookupCommit(oid)
				if err != nil {
					return
				}

				parentCommits = append(parentCommits, parentCommit)
			}
		}

		_, err = repo.CreateCommit("", author, committer, message, tree, parentCommits...)
		if err != nil {
			return
		}
	}
}

func push(srcRefName string, destRefName string, client redwood.HTTPClient) error {
	force := strings.HasPrefix(srcRefName, "+")
	if force {
		srcRefName = srcRefName[1:]
	}

	ref, err := repo.References.Dwim(srcRefName)
	if err != nil {
		return errors.WithStack(err)
	}
	headCommitId := ref.Target()

	stack := []*git.Oid{headCommitId}
	for len(stack) > 0 {
		commitId := stack[0]
		stack = stack[1:]

		txID := redwood.IDFromBytes(commitId[:])
		_, err := client.FetchTx(StateURI, txID)
		if err == redwood.Err404 {
			err = pushCommit(commitId, destRefName, client)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		commit, err := repo.LookupCommit(commitId)
		if err != nil {
			return err
		}

		parentCount := commit.ParentCount()
		for i := uint(0); i < parentCount; i++ {
			stack = append(stack, commit.ParentId(i))
		}
	}

	return pushRef(destRefName, headCommitId, client)
}

func pushRef(destRefName string, commitId *git.Oid, client redwood.HTTPClient) error {
	branchKeypath := append(RootKeypath, strings.Split(destRefName, "/")...)

	stateURI := StateURI + "-reflog"

	resp, _, parents, err := client.Get(stateURI, nil, branchKeypath, true)
	if err != nil {
		return err
	}
	defer resp.Close()

	var currentCommit string
	err = json.NewDecoder(resp).Decode(&currentCommit)
	if err != nil {
		return err
	}

	if currentCommit == "" {
		parents = []redwood.ID{redwood.GenesisTxID}
	}

	tx := &redwood.Tx{
		ID:      redwood.RandomID(),
		URL:     StateURI + "-reflog",
		From:    sigkeys.Address(),
		Parents: parents,
		Patches: []redwood.Patch{{
			Keys: branchKeypath,
			Val:  commitId.String(),
		}},
	}

	logf("reflog tx ~> %v", redwood.PrettyJSON(tx))

	return client.Put(tx)
}

func pushCommit(commitId *git.Oid, destRefName string, client redwood.HTTPClient) error {
	commit, err := repo.LookupCommit(commitId)
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
		hash        redwood.Hash
		mode        git.Filemode
	}
	type treeEntry struct {
		oid        *git.Oid
		objectType git.ObjectType
		fullPath   string
		mode       git.Filemode
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	chEntries := make(chan treeEntry)
	chUploaded := make(chan uploaded)
	chErr := make(chan error)
	var wg sync.WaitGroup

	go func() {
		defer close(chEntries)
		err = commitTree.Walk(func(rootPath string, entry *git.TreeEntry) int {
			select {
			case <-ctx.Done():
				return -1
			case chEntries <- treeEntry{entry.Id, entry.Type, filepath.Join(rootPath, entry.Name), entry.Filemode}:
			}
			return 0
		})
	}()

	for treeEntry := range chEntries {
		if treeEntry.objectType != git.ObjectBlob {
			continue
		}

		blob, err := repo.LookupBlob(treeEntry.oid)
		if err != nil {
			return errors.WithStack(err)
		}

		treeEntry := treeEntry
		wg.Add(1)
		go func() {
			defer wg.Done()

			data := blob.Contents()
			contentType, err := redwood.SniffContentType(treeEntry.fullPath, bytes.NewBuffer(data))
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case chErr <- err:
					return
				}
			}

			hash, err := client.StoreRef(bytes.NewReader(blob.Contents()))
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case chErr <- err:
					return
				}
			}
			select {
			case <-ctx.Done():
				return
			case chUploaded <- uploaded{treeEntry.fullPath, contentType, treeEntry.oid.String(), hash, treeEntry.mode}:
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
					cancel()
				case <-ctx.Done():
					return
				}
			}
		}()

		// @@TODO: merge commits with multiple parents
		var parentID redwood.ID
		if commit.ParentCount() == 0 {
			parentID = redwood.GenesisTxID
		} else {
			var err error
			parentID, err = redwood.IDFromHex(commit.ParentId(0).String())
			if err != nil {
				return
			}
		}

		commitID, err := redwood.IDFromHex(commit.Id().String())
		if err != nil {
			return
		}

		tx := &redwood.Tx{
			ID:         commitID,
			URL:        StateURI,
			From:       sigkeys.Address(),
			Parents:    []redwood.ID{parentID},
			Checkpoint: true,
		}

		tx.Patches = append(tx.Patches, redwood.Patch{
			Keys: []string{"message"},
			Val:  commit.Message(),
		})
		timeStr, err := commit.Time().MarshalText()
		if err != nil {
			return
		}
		tx.Patches = append(tx.Patches, redwood.Patch{
			Keys: []string{"timestamp"},
			Val:  string(timeStr),
		})
		author := commit.Author()
		authorTimeStr, err := author.When.MarshalText()
		if err != nil {
			return
		}
		tx.Patches = append(tx.Patches, redwood.Patch{
			Keys: []string{"author"},
			Val: M{
				"name":      author.Name,
				"email":     author.Email,
				"timestamp": string(authorTimeStr),
			},
		})
		committer := commit.Committer()
		committerTimeStr, err := committer.When.MarshalText()
		if err != nil {
			return
		}
		tx.Patches = append(tx.Patches, redwood.Patch{
			Keys: []string{"committer"},
			Val: M{
				"name":      committer.Name,
				"email":     committer.Email,
				"timestamp": string(committerTimeStr),
			},
		})

		fileTree := make(map[string]interface{})
		for uploaded := range chUploaded {
			select {
			case <-ctx.Done():
				return
			default:
			}
			fmt.Fprintf(os.Stderr, "uploaded: %+v\n", uploaded)
			setValueAtKeypath(fileTree, strings.Split(uploaded.name, "/"), M{
				"Content-Type": "link",
				"value":        "ref:" + uploaded.hash.Hex(),
				"mode":         int(uploaded.mode),
			}, true)
		}
		tx.Patches = append(tx.Patches, redwood.Patch{
			Keys: []string{"files"},
			Val:  fileTree,
		})

		err = client.Put(tx)
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

func getValue(x interface{}, keypath []string) (interface{}, bool) {
	for i := 0; i < len(keypath); i++ {
		if asMap, isMap := x.(map[string]interface{}); isMap {
			var exists bool
			x, exists = asMap[keypath[i]]
			if !exists {
				return nil, false
			}

		} else if asSlice, isSlice := x.([]interface{}); isSlice {
			sliceIdx, err := strconv.ParseInt(keypath[i], 10, 64)
			if err != nil {
				return nil, false
			} else if sliceIdx > int64(len(asSlice)-1) {
				return nil, false
			}
			x = asSlice[sliceIdx]

		} else {
			return nil, false
		}
	}
	return x, true
}

func getString(m interface{}, keypath []string) (string, bool) {
	x, exists := getValue(m, keypath)
	if !exists {
		return "", false
	}
	if s, isString := x.(string); isString {
		return s, true
	}
	return "", false
}

func getInt(m interface{}, keypath []string) (int, bool) {
	x, exists := getValue(m, keypath)
	if !exists {
		return 0, false
	}
	if i, isInt := x.(int); isInt {
		return i, true
	}
	return 0, false
}

func getFloat64(m interface{}, keypath []string) (float64, bool) {
	x, exists := getValue(m, keypath)
	if !exists {
		return 0, false
	}
	if f, isFloat := x.(float64); isFloat {
		return f, true
	}
	return 0, false
}

func getMap(m interface{}, keypath []string) (map[string]interface{}, bool) {
	x, exists := getValue(m, keypath)
	if !exists {
		return nil, false
	}
	if asMap, isMap := x.(map[string]interface{}); isMap {
		return asMap, true
	}
	return nil, false
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
