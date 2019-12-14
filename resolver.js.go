package redwood

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/pkg/errors"
	"rogchap.com/v8go"
)

type jsResolver struct {
	resolver
	vm            *v8go.Context
	internalState map[string]interface{}
}

func NewJSResolver(r RefResolver, config *NelSON, internalState map[string]interface{}) (Resolver, error) {
	resolvedConfig, anyMissing, err := r.ResolveRefs(config.Value())
	if err != nil {
		return nil, err
	} else if anyMissing {
		return nil, errors.WithStack(ErrMissingCriticalRefs)
	}

	srcval, exists := getValue(resolvedConfig, []string{"src"})
	if !exists {
		return nil, errors.New("js resolver needs a 'src' param")
	}

	readableSrc, ok := GetReadCloser(srcval)
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

	internalStateBytes, _ := json.Marshal(internalState)
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

func (r *jsResolver) ResolveState(state interface{}, sender Address, txID ID, parents []ID, patches []Patch) (newState interface{}, err error) {
	defer annotate(&err, "jsResolver.ResolveState")

	fmt.Printf("JS RESOLVER (tx.id = %v)\n", txID.Hex())

	convertedPatches := make([]interface{}, len(patches))
	for i, patch := range patches {
		convertedPatch := map[string]interface{}{
			"keys": patch.Keys,
			"val":  patch.Val,
		}

		if patch.Range != nil {
			convertedPatch["range"] = []interface{}{patch.Range.Start, patch.Range.End}
		}
		convertedPatches[i] = convertedPatch
	}

	stateJSON, _ := json.Marshal(state)
	var parentsArr []string
	for i := range parents {
		parentsArr = append(parentsArr, parents[i].String())
	}
	parentsArrJSON, _ := json.Marshal(parentsArr)
	convertedPatchesJSON, _ := json.Marshal(convertedPatches)

	script := "newStateJSON = global.resolve_state(" + string(stateJSON) + ", '" + sender.String() + "', '" + txID.String() + "', " + string(parentsArrJSON) + ", " + string(convertedPatchesJSON) + ")"
	_, err = r.vm.RunScript(script, "")
	if err != nil {
		return nil, err
	}

	newStateJSONVal, err := r.vm.RunScript("newStateJSON", "")
	if err != nil {
		return nil, err
	}

	var output map[string]interface{}
	err = json.Unmarshal([]byte(newStateJSONVal.String()), &output)
	if err != nil {
		return nil, err
	}

	r.internalState = output["internalState"].(map[string]interface{})

	return output["state"], nil
}
