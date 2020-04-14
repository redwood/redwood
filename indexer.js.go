package redwood

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/pkg/errors"
	"rogchap.com/v8go"

	"github.com/brynbellomy/redwood/nelson"
	"github.com/brynbellomy/redwood/tree"
)

type jsIndexer struct {
	vm *v8go.Context
}

func NewJSIndexer(config tree.Node) (Indexer, error) {
	srcval, exists, err := nelson.GetValueRecursive(config, tree.Keypath("src"), nil)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.Errorf("js indexer needs a 'src' param")
	}

	readableSrc, ok := nelson.GetReadCloser(srcval)
	if !ok {
		return nil, errors.Errorf("js indexer needs a 'src' param of type string, []byte, or io.ReadCloser (got %T)", srcval)
	}
	defer readableSrc.Close()

	srcStr, err := ioutil.ReadAll(readableSrc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ctx, _ := v8go.NewContext(nil)

	_, err = ctx.RunScript("var global = {}; var indexKey; "+string(srcStr), "")
	if err != nil {
		return nil, err
	}
	fmt.Println(string(srcStr))

	return &jsIndexer{vm: ctx}, nil
}

func (i *jsIndexer) IndexKeyForNode(node tree.Node) (_ tree.Keypath, err error) {
	defer withStack(&err)

	exists, err := node.Exists(nil)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}

	nodeJSON, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}

	script := "indexKey = global.indexKeyForNode(" + string(nodeJSON) + ")"
	_, err = i.vm.RunScript(script, "")
	if err != nil {
		return nil, err
	}

	indexKeyVal, err := i.vm.RunScript("indexKey", "")
	if err != nil {
		return nil, err
	}

	// The indexer can return undefined or null to indicate that this keypath shouldn't be indexed
	switch indexKeyVal.String() {
	case "undefined", "null":
		return nil, nil
	}

	var output string
	err = json.Unmarshal([]byte(indexKeyVal.String()), &output)
	if err != nil {
		return nil, err
	}

	return tree.Keypath(output), nil
}
