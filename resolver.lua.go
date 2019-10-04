package redwood

import (
	"reflect"

	"github.com/brynbellomy/go-luaconv"
	"github.com/yuin/gopher-lua"
)

type luaResolver struct {
	L *lua.LState
}

func NewLuaResolver(scriptSrc string) (*luaResolver, error) {
	L := lua.NewState()
	err := L.DoString(scriptSrc)
	if err != nil {
		return nil, err
	}
	return &luaResolver{L: L}, nil
}

func (r *luaResolver) ResolveState(state interface{}, patch Patch) (newState interface{}, err error) {
	defer annotate(&err, "luaResolver.ResolveState")

	luaState, err := luaconv.Wrap(r.L, reflect.ValueOf(state))
	if err != nil {
		return nil, err
	}

	luaPatch, err := luaconv.Wrap(r.L, reflect.ValueOf(patch))
	if err != nil {
		return nil, err
	}

	err = r.L.CallByParam(lua.P{
		Fn:      r.L.GetGlobal("resolve_state"),
		NRet:    0,
		Protect: false,
	}, luaState, luaPatch)
	if err != nil {
		return nil, err
	}
	return state, nil
}
