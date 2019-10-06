package redwood

import (
	"fmt"
	"reflect"

	"github.com/brynbellomy/go-luaconv"
	"github.com/pkg/errors"
	"github.com/yuin/gopher-lua"
)

type luaResolver struct {
	L  *lua.LState
	ID ID
}

func NewLuaResolver(params map[string]interface{}) (Resolver, error) {
	idx, _ := M(params).GetValue("id")
	var id ID
	if idx, is := idx.(ID); is {
		id = idx
	}

	src, exists := M(params).GetString("src")
	if !exists {
		return nil, errors.New("lua resolver needs a string 'src' param")
	}

	L := lua.NewState()
	err := L.DoString(src)
	if err != nil {
		return nil, err
	}
	return &luaResolver{L: L, ID: id}, nil
}

func (r *luaResolver) ResolveState(state interface{}, id ID, patch Patch) (newState interface{}, err error) {
	defer annotate(&err, "luaResolver.ResolveState")

	if r.ID.Pretty()[0] == '7' {
		fmt.Println("\n\n")
		fmt.Println("INCOMING STATE ~>", prettyJSON(state))
		fmt.Println("INCOMING PATCH ~>", prettyJSON(patch))
	}

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

	if r.ID.Pretty()[0] == '7' {
		fmt.Println("\n\n")
		fmt.Println("OUTGOING STATE ~>", prettyJSON(state))
	}

	return state, nil
}
