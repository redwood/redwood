package tree

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/wasi_snapshot_preview1"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/types"
)

type wasmResolver struct {
	log.Logger
	runtime        wazero.Runtime
	resolverModule api.Module
	internalState  map[string]interface{}
	node           state.Node
}

// Ensure jsResolver conforms to the Resolver interface
var _ Resolver = (*wasmResolver)(nil)

func NewWASMResolver(config state.Node, internalState map[string]interface{}) (_ Resolver, err error) {
	defer errors.Annotate(&err, "NewJSResolver")

	src, _, err := config.BytesValue(state.Keypath("src"))
	if err != nil {
		return nil, err
	} else if len(src) == 0 {
		return nil, errors.Errorf("wasm resolver needs a 'src' param")
	}

	runtime := wazero.NewRuntime()
	// defer runtime.Close(context.Background()) // This closes everything this Runtime created.

	_, err = wasi_snapshot_preview1.Instantiate(context.Background(), runtime)
	if err != nil {
		return nil, err
	}

	resolver := &wasmResolver{
		Logger:  log.NewLogger("resolver:wasm"),
		runtime: runtime,
	}

	_, err = runtime.NewModuleBuilder("env").
		ExportFunction("log", logString).
		ExportFunction("_set_state", func(ctx context.Context, m api.Module, jsonPtr, jsonLen uint32) {
			jsonBytes, ok := m.Memory().Read(ctx, jsonPtr, jsonLen)
			if !ok {
				panic("!ok")
			}

			type SetStateArgs struct {
				Keypath state.Keypath `json:"keypath"`
				Range   *state.Range  `json:"range"`
				Value   any           `json:"value"`
			}

			var args SetStateArgs
			err := json.Unmarshal(jsonBytes, &args)
			if err != nil {
				panic(err)
			}

			err = resolver.node.Set(args.Keypath, args.Range, args.Value)
			if err != nil {
				panic(fmt.Sprintf("!set: %v", err))
			}
		}).
		Instantiate(context.Background(), runtime)
	if err != nil {
		return nil, err
	}

	resolverModule, err := runtime.InstantiateModuleFromBinary(context.Background(), src)
	if err != nil {
		return nil, err
	}
	resolver.resolverModule = resolverModule

	// internalStateBytes, err := json.Marshal(internalState)
	// if err != nil {
	// 	return nil, err
	// }

	// internalStateScript := "init(" + string(internalStateBytes) + ")"
	// _, err = vm.RunScript(internalStateScript, "")
	// if err != nil {
	// 	return nil, err
	// }

	return resolver, nil
}

func (r *wasmResolver) InternalState() map[string]interface{} {
	return r.internalState
}

type WASMResolveArgs struct {
	Patches []WASMPatch `json:"patches"`
}

type WASMPatch struct {
	Keypath string      `json:"keypath"`
	Range   *WASMRange  `json:"range"`
	Value   interface{} `json:"value"`
}

type WASMRange struct {
	Start   uint64 `json:"start"`
	End     uint64 `json:"end"`
	Reverse bool   `json:"reverse"`
}

func (r *wasmResolver) ResolveState(node state.Node, blobStore blob.Store, sender types.Address, txID state.Version, parents []state.Version, patches []Patch) error {
	r.node = node

	malloc := r.resolverModule.ExportedFunction("allocate")
	free := r.resolverModule.ExportedFunction("deallocate")

	var buf bytes.Buffer

	wasmPatches := make([]WASMPatch, len(patches))
	for i, patch := range patches {
		var value interface{}
		err := json.Unmarshal(patch.ValueJSON, &value)
		if err != nil {
			return err
		}

		var rng *WASMRange
		if patch.Range != nil {
			rng = &WASMRange{
				Start:   patch.Range.Start,
				End:     patch.Range.End,
				Reverse: patch.Range.Reverse,
			}
		}
		wasmPatches[i] = WASMPatch{
			Keypath: string(patch.Keypath),
			Range:   rng,
			Value:   value,
		}
	}

	err := json.NewEncoder(&buf).Encode(WASMResolveArgs{
		Patches: wasmPatches,
	})
	if err != nil {
		return err
	}

	bs := buf.Bytes()

	results, err := malloc.Call(context.Background(), uint64(len(bs)))
	if err != nil {
		return err
	}
	argsPtr := results[0]
	defer free.Call(context.Background(), argsPtr)

	ok := r.resolverModule.Memory().Write(context.Background(), uint32(argsPtr), bs)
	if !ok {
		panic("could not write memory")
	}

	_, err = r.resolverModule.ExportedFunction("resolve_state").Call(context.Background(), argsPtr, uint64(len(bs)))

	return err
}

func logString(ctx context.Context, m api.Module, offset, byteCount uint32) {
	buf, ok := m.Memory().Read(ctx, offset, byteCount)
	if !ok {
		panic(fmt.Sprintf("Memory.Read(%d, %d) out of range", offset, byteCount))
	}
	fmt.Println(string(buf))
}
