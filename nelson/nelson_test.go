package nelson

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type M = map[string]interface{}

func setupDBTreeWithValue(T *testing.T, keypath tree.Keypath, val interface{}) (*tree.DBTree, types.ID) {
	i := rand.Int()
	state, err := tree.NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(T, err)
	v := types.RandomID()

	err = state.Update(v, func(tx *tree.DBNode) error {
		err := tx.Set(keypath, nil, val)
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)
	return state, v
}

func TestResolve_SimpleFrame(T *testing.T) {
	i := rand.Int()
	state, err := tree.NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(T, err)
	v := types.RandomID()

	expectedVal := M{
		"blah": "hi",
		"asdf": []interface{}{uint64(5), uint64(4), uint64(3)},
	}

	err = state.Update(v, func(tx *tree.DBNode) error {
		err := tx.Set(nil, nil, M{
			"Content-Type":   "some content type",
			"Content-Length": int64(111),
			//"value":          "abcdefg",
			"value": expectedVal,
		})
		require.NoError(T, err)

		return nil
	})
	require.NoError(T, err)

	refResolver := &refResolverMock{}

	err = state.View(v, func(tx *tree.DBNode) error {
		memstate, err := tx.CopyToMemory(nil, nil)
		require.NoError(T, err)

		memstate.(*tree.MemoryNode).DebugPrint()

		anyMissing, err := Resolve(memstate, refResolver)
		require.False(T, anyMissing)
		require.NoError(T, err)

		memstate.(*tree.MemoryNode).DebugPrint()

		val, exists, err := memstate.Value(nil, nil)
		require.True(T, exists)
		require.NoError(T, err)

		asNelSON, isNelSON := val.(*Frame)
		require.True(T, isNelSON)

		require.Equal(T, "some content type", asNelSON.ContentType())
		require.Equal(T, int64(111), asNelSON.ContentLength())

		val, exists, err = asNelSON.Value()
		require.True(T, exists)
		require.NoError(T, err)
		require.Equal(T, expectedVal, val)

		return nil
	})
}

func xTestResolve_SimpleFrameInFrame(T *testing.T) {
	i := rand.Int()
	state, err := tree.NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(T, err)
	v := types.RandomID()

	err = state.Update(v, func(tx *tree.DBNode) error {
		err := tx.Set(tree.Keypath("foo"), nil, M{
			"blah": M{
				"Content-Type":   "outer",
				"Content-Length": int64(111),
				"value": M{
					"Content-Length": int64(4321),
					"value": M{
						"Content-Type": "inner",
						"value":        uint64(12345),
					},
				},
			},
		})
		require.NoError(T, err)

		return nil
	})
	require.NoError(T, err)

	refResolver := &refResolverMock{}

	err = state.View(v, func(tx *tree.DBNode) error {
		memstate, err := tx.CopyToMemory(nil, nil)
		require.NoError(T, err)

		anyMissing, err := Resolve(memstate, refResolver)
		require.False(T, anyMissing)
		require.NoError(T, err)

		val, exists, err := memstate.Value(tree.Keypath("foo/blah"), nil)
		require.True(T, exists)
		require.NoError(T, err)

		asNelSON, isNelSON := val.(*Frame)
		require.True(T, isNelSON)

		require.Equal(T, "inner", asNelSON.ContentType())
		require.Equal(T, int64(4321), asNelSON.ContentLength())

		val, exists, err = asNelSON.Value()
		require.True(T, exists)
		require.NoError(T, err)
		require.Equal(T, uint64(12345), val)

		return nil
	})
}

func TestResolve_LinkToSimpleState(T *testing.T) {
	localState, v := setupDBTreeWithValue(T, tree.Keypath("foo"), M{
		"blah": M{
			"Content-Type": "outer",
			"value": M{
				"Content-Length": float64(4321),
				"value": M{
					"Content-Type": "link",
					"value":        "state:otherState/someChannel/foo/xyzzy/zork",
				},
			},
		},
	})
	defer localState.DeleteDB()

	otherState, v2 := setupDBTreeWithValue(T, tree.Keypath("foo"), M{
		"xyzzy": M{
			"zork": uint64(55555),
		},
	})
	defer otherState.DeleteDB()

	err := localState.View(v, func(localState *tree.DBNode) error {
		return otherState.View(v2, func(otherState *tree.DBNode) error {
			refResolver := &refResolverMock{
				stateURIs: map[string]tree.Node{
					"otherState/someChannel": otherState,
				},
			}

			localStateMem, err := localState.CopyToMemory(nil, nil)
			require.NoError(T, err)

			anyMissing, err := Resolve(localStateMem, refResolver)
			require.False(T, anyMissing)
			require.NoError(T, err)

			val, exists, err := localStateMem.Value(tree.Keypath("foo/blah"), nil)
			require.True(T, exists)
			require.NoError(T, err)

			asNelSON, isNelSON := val.(*Frame)
			require.True(T, isNelSON)

			val, exists, err = asNelSON.Value()
			require.True(T, exists)
			require.NoError(T, err)
			require.Equal(T, uint64(55555), val)
			return nil
		})
	})
	require.NoError(T, err)
}

func TestResolve_LinkToMatryoshkaState(T *testing.T) {
	localState, v := setupDBTreeWithValue(T, tree.Keypath("foo"), M{
		"blah": M{
			"Content-Type": "ignore this",
			"value": M{
				"Content-Length": int64(4321),
				"value": M{
					"Content-Type": "link",
					"value":        "state:otherState/someChannel/foo/xyzzy/zork",
				},
			},
		},
	})
	defer localState.DeleteDB()

	otherState, v2 := setupDBTreeWithValue(T, tree.Keypath("foo"), M{
		"xyzzy": M{
			"zork": M{
				"Content-Type":   "linked content type",
				"Content-Length": int64(9999),
				"value":          uint64(55555),
			},
		},
	})
	defer otherState.DeleteDB()

	err := localState.View(v, func(localState *tree.DBNode) error {
		return otherState.View(v2, func(otherState *tree.DBNode) error {
			refResolver := &refResolverMock{
				stateURIs: map[string]tree.Node{
					"otherState/someChannel": otherState,
				},
			}

			localStateMem, err := localState.CopyToMemory(nil, nil)
			require.NoError(T, err)

			anyMissing, err := Resolve(localStateMem, refResolver)
			require.False(T, anyMissing)
			require.NoError(T, err)

			val, exists, err := localStateMem.Value(tree.Keypath("foo/blah"), nil)
			require.True(T, exists)
			require.NoError(T, err)

			asNelSON, isNelSON := val.(*Frame)
			require.True(T, isNelSON)

			//require.Equal(T, "linked content type", asNelSON.ContentType())
			//require.Equal(T, int64(9999), asNelSON.ContentLength())

			val, exists, err = asNelSON.Value()
			require.True(T, exists)
			require.NoError(T, err)
			require.Equal(T, uint64(55555), val)
			return nil
		})
	})
	require.NoError(T, err)
}

func PrettyJSON(x interface{}) string {
	j, _ := json.MarshalIndent(x, "", "    ")
	return string(j)
}

//func debugPrint_DBNode(T *testing.T, state *tree.DBTree) {
//    keypaths, values, err := state.Contents(nil, nil)
//    require.NoError(T, err)
//    for i := range keypaths {
//        fmt.Println("  -", string(keypaths[i]), values[i])
//    }
//}

//
//func debugPrint_MemoryNode(T *testing.T, memstate interface{}) {
//    keypaths, values, err := memstate.(interface {
//        Contents(keypathPrefix tree.Keypath, rng *[2]uint64) ([]tree.Keypath, []interface{}, error)
//    }).Contents(nil, nil)
//
//    require.NoError(T, err)
//
//    for i := range keypaths {
//        fmt.Println(" >>", string(keypaths[i]), values[i])
//    }
//}
