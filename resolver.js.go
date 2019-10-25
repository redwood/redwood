package redwood

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
)

type jsResolver struct {
	vm *otto.Otto
}

func NewJSResolver(params map[string]interface{}) (Resolver, error) {
	src, exists := M(params).GetString("src")
	if !exists {
		return nil, errors.New("js resolver needs a string 'src' param")
	}

	vm := otto.New()
	vm.Run(src)

	return &jsResolver{vm: vm}, nil
}

func (r *jsResolver) ResolveState(state interface{}, sender Address, patch Patch) (newState interface{}, err error) {
	defer annotate(&err, "jsResolver.ResolveState")

	var rangeAsMap map[string]interface{}
	if patch.Range != nil {
		rangeAsMap = make(map[string]interface{})
		rangeAsMap["start"] = patch.Range.Start
		rangeAsMap["end"] = patch.Range.End
	}

	patchAsMap := map[string]interface{}{
		"keys":  patch.Keys,
		"range": rangeAsMap,
		"val":   patch.Val,
	}

	stateBytes, _ := json.Marshal(state)

	val, err := r.vm.Call("resolve_state", nil, string(stateBytes), sender.String(), patchAsMap)
	if err != nil {
		return nil, err
	}

	x, err := val.Export()
	if err != nil {
		panic(err)
	}

	var nextState map[string]interface{}
	err = json.Unmarshal([]byte(x.(string)), &nextState)
	if err != nil {
		panic(err)
	}

	return nextState, nil
}
