//go:build otto || raspi

package tree

import (
	"encoding/json"
	"io/ioutil"

	"github.com/deoxxa/otto"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/tree/nelson"
	"redwood.dev/types"
)

type jsResolver struct {
	log.Logger
	vm            *otto.Otto
	internalState map[string]interface{}
}

// Ensure jsResolver conforms to the Resolver interface
var _ Resolver = (*jsResolver)(nil)

func NewJSResolver(config state.Node, internalState map[string]interface{}) (_ Resolver, err error) {
	defer errors.Annotate(&err, "NewJSResolver")

	// srcval, exists, err := nelson.GetValueRecursive(config, state.Keypath("src"), nil)
	srcval, exists, err := config.Value(state.Keypath("src"), nil)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.Errorf("js resolver needs a 'src' param")
	}

	readableSrc, ok := nelson.GetReadCloser(srcval)
	if !ok {
		return nil, errors.Errorf("js resolver needs a 'src' param of type string, []byte, or io.ReadCloser (got %T)", srcval)
	}
	defer readableSrc.Close()

	srcStr, err := ioutil.ReadAll(readableSrc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	vm := otto.New()
	vm.Set("global", map[string]interface{}{})

	_, err = vm.Run(srcStr)
	if err != nil {
		return nil, err
	}

	internalStateBytes, err := json.Marshal(internalState)
	if err != nil {
		return nil, err
	}

	_, err = vm.Call("global.init", nil, string(internalStateBytes))
	if err != nil {
		return nil, err
	}

	return &jsResolver{Logger: log.NewLogger("resolver:js"), vm: vm, internalState: internalState}, nil
}

func (r *jsResolver) InternalState() map[string]interface{} {
	return r.internalState
}

func (r *jsResolver) ResolveState(node state.Node, blobStore blob.Store, sender types.Address, txID state.Version, parents []state.Version, patches []Patch) (err error) {
	defer errors.Annotate(&err, "jsResolver.ResolveState")

	convertedPatches := make([]interface{}, len(patches))
	for i, patch := range patches {
		var value interface{}
		err = json.Unmarshal(patch.ValueJSON, &value)
		if err != nil {
			return err
		}

		convertedPatch := map[string]interface{}{
			"keys": patch.Keypath.PartStrings(),
			"val":  value,
		}

		if patch.Range != nil {
			convertedPatch["range"] = []interface{}{patch.Range.Start, patch.Range.End}
		}
		convertedPatches[i] = convertedPatch
	}

	stateJSON, err := json.Marshal(node)
	if err != nil {
		return err
	}

	var parentsArr []string
	for i := range parents {
		parentsArr = append(parentsArr, parents[i].String())
	}

	val, err := r.vm.Call("global.resolve_state", nil, string(stateJSON), sender.String(), txID.String(), parentsArr, convertedPatches)
	if err != nil {
		return err
	}

	x, err := val.Export()
	if err != nil {
		return err
	}

	var output map[string]interface{}
	err = json.Unmarshal([]byte(x.(string)), &output)
	if err != nil {
		return err
	}

	r.internalState = output["internalState"].(map[string]interface{})
	node.Set(nil, nil, output["state"])
	return nil
}
