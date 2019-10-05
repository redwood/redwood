package redwood

import (
	"io/ioutil"
	"reflect"

	"github.com/brynbellomy/go-luaconv"
	"github.com/yuin/gopher-lua"
)

type luaResolver struct {
	L *lua.LState
}

func NewLuaResolverFromFile(filename string) (*luaResolver, error) {
	bs, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return NewLuaResolver(string(bs))
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
	}, luaState, luaPatch)
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
