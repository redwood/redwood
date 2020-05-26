package redwood

import (
	"io/ioutil"
	"reflect"

	"github.com/brynbellomy/go-luaconv"
	"github.com/pkg/errors"
	lua "github.com/yuin/gopher-lua"

	"github.com/brynbellomy/redwood/nelson"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type luaResolver struct {
	L *lua.LState
}

func NewLuaResolver(config tree.Node, internalState map[string]interface{}) (Resolver, error) {
	srcval, exists, err := nelson.GetValueRecursive(config, tree.Keypath("src"), nil)
	if err != nil {
		return nil, errors.WithStack(err)
	} else if !exists {
		return nil, errors.Errorf("lua resolver needs a 'src' param")
	}

	readableSrc, ok := nelson.GetReadCloser(srcval)
	if !ok {
		return nil, errors.Errorf("lua resolver needs a 'src' param of type string, []byte, or io.ReadCloser (got %T)", srcval)
	}
	defer readableSrc.Close()

	srcStr, err := ioutil.ReadAll(readableSrc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	L := lua.NewState()
	err = L.DoString(string(srcStr))
	if err != nil {
		return nil, err
	}
	return &luaResolver{L: L}, nil
}

func (r *luaResolver) InternalState() map[string]interface{} {
	return nil
}

func (r *luaResolver) ResolveState(state tree.Node, refStore RefStore, sender types.Address, txID types.ID, parents []types.ID, patches []Patch) (err error) {
	defer annotate(&err, "luaResolver.ResolveState")

	luaPatches, err := luaconv.Wrap(r.L, reflect.ValueOf(patches))
	if err != nil {
		return errors.WithStack(err)
	}

	luaState, err := luaconv.Wrap(r.L, reflect.ValueOf(state))
	if err != nil {
		return errors.WithStack(err)
	}

	err = r.L.CallByParam(lua.P{
		Fn:      r.L.GetGlobal("resolve_state"),
		NRet:    1,
		Protect: true,
	}, luaState, lua.LString(sender.String()), luaPatches)
	if err != nil {
		return errors.WithStack(err)
	}

	retval := r.L.Get(-1)
	// @@TODO: rewrite all of this
	_, err = luaconv.Decode(retval, reflect.TypeOf(map[string]interface{}{}))
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
