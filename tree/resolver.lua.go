package tree

import (
	"reflect"

	"github.com/brynbellomy/go-luaconv"
	lua "github.com/yuin/gopher-lua"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/types"
)

type luaResolver struct {
	L *lua.LState
}

func NewLuaResolver(config state.Node, internalState map[string]interface{}) (Resolver, error) {
	src, is, err := config.NodeAt(state.Keypath("src"), nil).StringValue(nil)
	if err != nil {
		return nil, err
	} else if !is {
		return nullResolver{}, nil
	}

	L := lua.NewState()
	err = L.DoString(src)
	if err != nil {
		return nil, err
	}
	return &luaResolver{L: L}, nil
}

func (r *luaResolver) InternalState() map[string]interface{} {
	return nil
}

func (r *luaResolver) ResolveState(node state.Node, blobStore blob.Store, sender types.Address, txID state.Version, parents []state.Version, patches []Patch) (err error) {
	defer errors.Annotate(&err, "luaResolver.ResolveState")

	luaPatches, err := luaconv.Wrap(r.L, reflect.ValueOf(patches))
	if err != nil {
		return err
	}

	luaState, err := luaconv.Wrap(r.L, reflect.ValueOf(node))
	if err != nil {
		return err
	}

	err = r.L.CallByParam(lua.P{
		Fn:      r.L.GetGlobal("resolve_state"),
		NRet:    1,
		Protect: true,
	}, luaState, lua.LString(sender.String()), luaPatches)
	if err != nil {
		return err
	}
	return nil
}
