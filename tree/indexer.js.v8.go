//go:build !otto && !raspi
package tree

// import (
// 	"encoding/json"
// 	"fmt"

// 	"rogchap.com/v8go"

// 	"redwood.dev/errors"
// 	"redwood.dev/state"
// )

// type jsIndexer struct {
// 	vm *v8go.Context
// }

// func NewJSIndexer(config state.Node) (Indexer, error) {
// 	src, is, err := config.NodeAt(state.Keypath("src")).StringValue()
// 	if err != nil {
// 		return nil, err
// 	} else if !is {
// 		return nullResolver{}, nil
// 	}

// 	ctx, _ := v8go.NewContext(nil)

// 	_, err = ctx.RunScript("var global = {}; var result; "+string(srcStr), "")
// 	if err != nil {
// 		return nil, err
// 	}
// 	fmt.Println(string(srcStr))

// 	return &jsIndexer{vm: ctx}, nil
// }

// func (i *jsIndexer) IndexNode(relKeypath state.Keypath, node state.Node) (_ state.Keypath, _ state.Node, err error) {
// 	defer errors.AddStack(&err)

// 	exists, err := node.Exists(nil)
// 	if err != nil {
// 		return nil, nil, err
// 	} else if !exists {
// 		return nil, nil, nil
// 	}

// 	nodeJSON, err := json.Marshal(node)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	script := `result = JSON.stringify(global.indexNode("` + relKeypath.String() + `", ` + string(nodeJSON) + `))`
// 	_, err = i.vm.RunScript(script, "")
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	resultVal, err := i.vm.RunScript("result", "")
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	// The indexer can return undefined or null to indicate that this keypath shouldn't be indexed
// 	switch resultVal.String() {
// 	case "undefined", "null":
// 		return nil, nil, nil
// 	}

// 	var output [2]interface{}
// 	err = json.Unmarshal([]byte(resultVal.String()), &output)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	indexKey, ok := output[0].(string)
// 	if !ok {
// 		return nil, nil, errors.New("index key must be a string")
// 	}

// 	nodeToIndex := state.NewMemoryNode()
// 	err = nodeToIndex.Set(nil, nil, output[1])
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	return state.Keypath(indexKey), nodeToIndex, nil
// }
