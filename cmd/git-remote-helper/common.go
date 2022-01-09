package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	git "github.com/brynbellomy/git2go"

	"redwood.dev/state"
)

type Commit struct {
	Hash      string            `json:"-"`
	Parents   []string          `json:"parents"`
	Message   string            `json:"message"`
	Timestamp time.Time         `json:"timestamp"`
	Author    AuthorCommitter   `json:"author"`
	Committer AuthorCommitter   `json:"committer"`
	Files     *state.MemoryNode `json:"files"`
	Signature string            `json:"sig"`
}

type AuthorCommitter struct {
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
}

type FileData struct {
	ContentType   string       `json:"Content-Type"`
	ContentLength uint64       `json:"Content-Length"`
	Value         string       `json:"value"`
	SHA1          string       `json:"sha1"`
	Mode          git.Filemode `json:"mode"`
}

func die(err error) {
	logf("error: %+v", err)
	os.Exit(1)
}

func logf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

func sayln(w io.Writer, args ...interface{}) {
	fmt.Fprintln(w, args...)
	args = append([]interface{}{">>>"}, args...)
	fmt.Fprintln(os.Stderr, args...)
}

func mustJSON(x interface{}) []byte {
	bs, err := json.Marshal(x)
	if err != nil {
		panic(err)
	}
	return bs
}

func walkFiles(n interface{}, parentKeypath state.Keypath, fn func(node interface{}, fileData FileData, keypath state.Keypath) error) error {
	bs, err := json.Marshal(n)
	if err != nil {
		return err
	}

	m := map[string]interface{}{}
	err = json.Unmarshal(bs, &m)
	if err != nil {
		return err
	}

	for filepath, stuff := range m {
		stuffMap, isMap := stuff.(map[string]interface{})
		if !isMap {
			continue
		}

		thisKeypath := parentKeypath.Copy().Pushs(filepath)

		if stuffMap["Content-Type"] == nil {
			err := walkFiles(stuffMap, thisKeypath, fn)
			if err != nil {
				return err
			}
			continue
		}

		bs, err := json.Marshal(stuffMap)
		if err != nil {
			return err
		}

		var fileData FileData
		err = json.Unmarshal(bs, &fileData)
		if err != nil {
			return err
		}

		err = fn(stuffMap, fileData, thisKeypath)
		if err != nil {
			return err
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
