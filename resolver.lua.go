package redwood

import (
	"reflect"

	"github.com/brynbellomy/go-luaconv"
	"github.com/pkg/errors"
	lua "github.com/yuin/gopher-lua"
)

type luaResolver struct {
	resolver
	L *lua.LState
}

func NewLuaResolver(params map[string]interface{}, internalState map[string]interface{}) (Resolver, error) {
	src, exists := getString(params, []string{"src"})
	if !exists {
		return nil, errors.New("lua resolver needs a string 'src' param")
	}

	L := lua.NewState()
	err := L.DoString(src)
	if err != nil {
		return nil, err
	}
	return &luaResolver{L: L}, nil
}

func (r *luaResolver) InternalState() map[string]interface{} {
	return nil
}

func (r *luaResolver) ResolveState(state interface{}, sender Address, txID ID, parents []ID, patches []Patch) (newState interface{}, err error) {
	defer annotate(&err, "luaResolver.ResolveState")

	luaPatches, err := luaconv.Wrap(r.L, reflect.ValueOf(patches))
	if err != nil {
		return nil, err
	}

	luaState, err := luaconv.Encode(r.L, reflect.ValueOf(state))
	if err != nil {
		return nil, err
	}

	err = r.L.CallByParam(lua.P{
		Fn:      r.L.GetGlobal("resolve_state"),
		NRet:    1,
		Protect: true,
	}, luaState, lua.LString(sender.String()), luaPatches)
	if err != nil {
		return nil, err
	}

	retval := r.L.Get(-1)
	rval, err := luaconv.Decode(retval, reflect.TypeOf(map[string]interface{}{}))
	if err != nil {
		panic(err)
	}
	state = rval.Interface()

	return state, nil
}
