package redwood

import (
	"reflect"

	"github.com/brynbellomy/go-luaconv"
	"github.com/pkg/errors"
	"github.com/yuin/gopher-lua"
)

type luaResolver struct {
	L *lua.LState
}

func NewLuaResolver(params map[string]interface{}) (Resolver, error) {
	src, exists := M(params).GetString("src")
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

func (r *luaResolver) ResolveState(state interface{}, id ID, patch Patch) (newState interface{}, err error) {
	defer annotate(&err, "luaResolver.ResolveState")

	luaState, err := luaconv.Encode(r.L, reflect.ValueOf(state))
	if err != nil {
		return nil, err
	}

	luaPatch, err := luaconv.Wrap(r.L, reflect.ValueOf(patch))
	if err != nil {
		return nil, err
	}

	err = r.L.CallByParam(lua.P{
		Fn:      r.L.GetGlobal("resolve_state"),
		NRet:    1,
		Protect: false,
	}, luaState, lua.LString(id.Pretty()), luaPatch)
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
