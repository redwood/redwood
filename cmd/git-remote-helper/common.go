package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/tree/nelson"
)

type Commit struct {
	Hash      string            `json:"-"`
	Parents   []string          `json:"parents"`
	Message   string            `json:"message"`
	Timestamp time.Time         `json:"timestamp"`
	Author    AuthorCommitter   `json:"author"`
	Committer AuthorCommitter   `json:"committer"`
	Files     *state.MemoryNode `json:"files"`
}

type AuthorCommitter struct {
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
}

func die(err error) {
	logf("error: %+v", err)
	os.Exit(1)
}

func logf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

func mustJSON(x interface{}) []byte {
	bs, err := json.Marshal(x)
	if err != nil {
		panic(err)
	}
	return bs
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
