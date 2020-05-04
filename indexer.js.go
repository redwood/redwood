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

	_, err = ctx.RunScript("var global = {}; var result; "+string(srcStr), "")
	if err != nil {
		return nil, err
	}
	fmt.Println(string(srcStr))

	return &jsIndexer{vm: ctx}, nil
}

func (i *jsIndexer) IndexNode(relKeypath tree.Keypath, node tree.Node) (_ tree.Keypath, _ tree.Node, err error) {
	defer withStack(&err)

	exists, err := node.Exists(nil)
	if err != nil {
		return nil, nil, err
	} else if !exists {
		return nil, nil, nil
	}

	nodeJSON, err := json.Marshal(node)
	if err != nil {
		return nil, nil, err
	}

	script := `result = JSON.stringify(global.indexNode("` + relKeypath.String() + `", ` + string(nodeJSON) + `))`
	_, err = i.vm.RunScript(script, "")
	if err != nil {
		return nil, nil, err
	}

	resultVal, err := i.vm.RunScript("result", "")
	if err != nil {
		return nil, nil, err
	}

	// The indexer can return undefined or null to indicate that this keypath shouldn't be indexed
	switch resultVal.String() {
	case "undefined", "null":
		return nil, nil, nil
	}

	var output [2]interface{}
	err = json.Unmarshal([]byte(resultVal.String()), &output)
	if err != nil {
		return nil, nil, err
	}

	indexKey, ok := output[0].(string)
	if !ok {
		return nil, nil, errors.New("index key must be a string")
	}

	nodeToIndex := tree.NewMemoryNode()
	err = nodeToIndex.Set(nil, nil, output[1])
	if err != nil {
		return nil, nil, err
	}

	return tree.Keypath(indexKey), nodeToIndex, nil
}
