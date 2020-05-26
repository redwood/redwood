package redwood

import (
	"encoding/json"
	"io/ioutil"

	"github.com/pkg/errors"
	"rogchap.com/v8go"
	//"github.com/saibing/go-v8"

	"github.com/brynbellomy/redwood/nelson"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type jsResolver struct {
	vm            *v8go.Context
	internalState map[string]interface{}
}

// Ensure jsResolver conforms to the Resolver interface
var _ Resolver = (*jsResolver)(nil)

func NewJSResolver(config tree.Node, internalState map[string]interface{}) (_ Resolver, err error) {
	defer annotate(&err, "NewJSResolver")

	// srcval, exists, err := nelson.GetValueRecursive(config, tree.Keypath("src"), nil)
	srcval, exists, err := config.Value(tree.Keypath("src"), nil)
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

	ctx, _ := v8go.NewContext(nil)

	_, err = ctx.RunScript("var global = {}; var newStateJSON; "+string(srcStr), "")
	if err != nil {
		return nil, err
	}

	internalStateBytes, err := json.Marshal(internalState)
	if err != nil {
		return nil, err
	}

	internalStateScript := "global.init(" + string(internalStateBytes) + ")"
	_, err = ctx.RunScript(internalStateScript, "")
	if err != nil {
		return nil, err
	}

	return &jsResolver{vm: ctx, internalState: internalState}, nil
}

func (r *jsResolver) InternalState() map[string]interface{} {
	return r.internalState
}

func (r *jsResolver) ResolveState(state tree.Node, refStore RefStore, sender types.Address, txID types.ID, parents []types.ID, patches []Patch) (err error) {
	defer annotate(&err, "jsResolver.ResolveState")

	convertedPatches := make([]interface{}, len(patches))
	for i, patch := range patches {
		convertedPatch := map[string]interface{}{
			"keys": patch.Keypath.PartStrings(),
			"val":  patch.Val,
		}

		if patch.Range != nil {
			convertedPatch["range"] = []interface{}{patch.Range[0], patch.Range[1]}
		}
		convertedPatches[i] = convertedPatch
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return err
	}

	var parentsArr []string
	for i := range parents {
		parentsArr = append(parentsArr, parents[i].String())
	}
	parentsArrJSON, _ := json.Marshal(parentsArr)
	convertedPatchesJSON, _ := json.Marshal(convertedPatches)

	script := "newStateJSON = global.resolve_state(" + string(stateJSON) + ", '" + sender.String() + "', '" + txID.String() + "', " + string(parentsArrJSON) + ", " + string(convertedPatchesJSON) + ")"
	_, err = r.vm.RunScript(script, "")
	if err != nil {
		return err
	}

	newStateJSONVal, err := r.vm.RunScript("newStateJSON", "")
	if err != nil {
		return err
	}

	var output map[string]interface{}
	err = json.Unmarshal([]byte(newStateJSONVal.String()), &output)
	if err != nil {
		return err
	}

	r.internalState = output["internalState"].(map[string]interface{})

	state.Set(nil, nil, output["state"])

	return nil
}
