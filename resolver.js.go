package redwood

import (
	"encoding/json"

	"github.com/deoxxa/otto"
	"github.com/pkg/errors"
)

type jsResolver struct {
	resolver
	vm            *otto.Otto
	internalState map[string]interface{}
}

func NewJSResolver(params map[string]interface{}, internalState map[string]interface{}) (Resolver, error) {
	src, exists := M(params).GetString("src")
	if !exists {
		return nil, errors.New("js resolver needs a string 'src' param")
	}

	vm := otto.New()
	vm.Set("global", map[string]interface{}{})

	_, err := vm.Run(src)
	if err != nil {
		return nil, err
	}

	internalStateBytes, _ := json.Marshal(internalState)
	_, err = vm.Call("global.init", nil, string(internalStateBytes))
	if err != nil {
		return nil, err
	}

	return &jsResolver{vm: vm, internalState: internalState}, nil
}

func (r *jsResolver) InternalState() map[string]interface{} {
	return r.internalState
}

func (r *jsResolver) ResolveState(state interface{}, sender Address, txID ID, parents []ID, patches []Patch) (newState interface{}, err error) {
	defer annotate(&err, "jsResolver.ResolveState")

	convertedPatches := make([]interface{}, len(patches))
	for i, patch := range patches {
		var rangeAsSlice []interface{}
		if patch.Range != nil {
			rangeAsSlice = []interface{}{patch.Range.Start, patch.Range.End}
		}

		convertedPatches[i] = map[string]interface{}{
			"keys":  patch.Keys,
			"range": rangeAsSlice,
			"val":   patch.Val,
		}
	}

	stateBytes, _ := json.Marshal(state)
	var parentsArr []string
	for i := range parents {
		parentsArr = append(parentsArr, parents[i].String())
	}

	val, err := r.vm.Call("global.resolve_state", nil, string(stateBytes), sender.String(), txID.String(), parentsArr, convertedPatches)
	if err != nil {
		return nil, err
	}

	x, err := val.Export()
	if err != nil {
		panic(err)
	}

	var output map[string]interface{}
	err = json.Unmarshal([]byte(x.(string)), &output)
	if err != nil {
		panic(err)
	}

	r.internalState = output["internalState"].(map[string]interface{})

	return output["state"], nil
}
