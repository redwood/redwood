//go:build otto || raspi
package tree

import (
	"encoding/json"
	"io/ioutil"

	"github.com/deoxxa/otto"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/tree/nelson"
)

type jsIndexer struct {
	vm *otto.Otto
}

func NewJSIndexer(config state.Node) (Indexer, error) {
	srcval, exists, err := nelson.GetValueRecursive(config, state.Keypath("src"), nil)
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

	vm := otto.New()
	vm.Set("global", map[string]interface{}{})

	_, err = vm.Run("var global = {}; var result; " + string(srcStr))
	if err != nil {
		return nil, err
	}

	return &jsIndexer{vm: vm}, nil
}

func (i *jsIndexer) IndexNode(relKeypath state.Keypath, node state.Node) (_ state.Keypath, _ state.Node, err error) {
	defer errors.AddStack(&err)

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
	val, err := i.vm.Run(script)
	if err != nil {
		return nil, nil, err
	}

	// The indexer can return undefined or null to indicate that this keypath shouldn't be indexed
	if val.IsNull() || val.IsUndefined() {
		return nil, nil, nil
	}

	goVal, err := val.Export()
	if err != nil {
		return nil, nil, err
	}

	results, is := goVal.([]interface{})
	if !is || len(results) != 2 {
		return nil, nil, errors.New("indexer must return [indexKey: string, indexNode: any]")
	}

	indexKey, is := results[0].(string)
	if !is {
		return nil, nil, errors.New("indexer must return [indexKey: string, indexNode: any]")
	}

	nodeToIndex := state.NewMemoryNode()
	err = nodeToIndex.Set(nil, nil, results[1])
	if err != nil {
		return nil, nil, err
	}

	return state.Keypath(indexKey), nodeToIndex, nil
}