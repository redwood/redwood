package tree

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/brynbellomy/redwood/types"
)

func TestDBTree_CopyToMemory(T *testing.T) {
	//T.Parallel()
	//
	//i := rand.Int()
	//tree, err := NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	//require.NoError(T, err)
	//defer tree.DeleteDB()
	//
	//err = tree.Update(func(tx *DBNode) error {
	//    _, err := tx.Set(nil, nil, testVal1)
	//    require.NoError(T, err)
	//    return nil
	//})
	//require.NoError(T, err)
	//
	//expected := []struct {
	//    keypath  Keypath
	//    nodeType NodeType
	//    val      interface{}
	//}{
	//    {Keypath(""), NodeTypeMap, testVal1},
	//    {Keypath("hello"), NodeTypeMap, testVal1["hello"]},
	//    {Keypath("hello/xyzzy"), NodeTypeValue, testVal1["hello"].(M)["xyzzy"]},
	//    {Keypath("flox"), NodeTypeSlice, testVal1["flox"]},
	//    {Keypath("flox").PushIndex(0), NodeTypeValue, testVal1["flox"].([]interface{})[0]},
	//    {Keypath("flox").PushIndex(1), NodeTypeMap, testVal1["flox"].([]interface{})[1]},
	//    {Keypath("flox").PushIndex(1).Push(Keypath("yup")), NodeTypeValue, testVal1["flox"].([]interface{})[1].(M)["yup"]},
	//    {Keypath("flox").PushIndex(1).Push(Keypath("hey")), NodeTypeValue, testVal1["flox"].([]interface{})[1].(M)["hey"]},
	//    {Keypath("flox").PushIndex(2), NodeTypeValue, testVal1["flox"].([]interface{})[2]},
	//}
	//
	//expectedValues := map[string]interface{}{
	//    "":                                   testVal1,
	//    "hello":                              testVal1["hello"],
	//    "hello/xyzzy":                        testVal1["hello"].(M)["xyzzy"],
	//    "flox":                               testVal1["flox"],
	//    string(Keypath("flox").PushIndex(0)): testVal1["flox"].([]interface{})[0],
	//    string(Keypath("flox").PushIndex(1)): testVal1["flox"].([]interface{})[1],
	//    string(Keypath("flox").PushIndex(1).Push(Keypath("yup"))): testVal1["flox"].([]interface{})[1].(M)["yup"],
	//    string(Keypath("flox").PushIndex(1).Push(Keypath("hey"))): testVal1["flox"].([]interface{})[1].(M)["hey"],
	//    string(Keypath("flox").PushIndex(2)):                      testVal1["flox"].([]interface{})[2],
	//}
	//
	//sort.Slice(expectedKeypaths, func(i, j int) bool { return bytes.Compare(expectedKeypaths[i], expectedKeypaths[j]) < 0 })
	//
	//copied, err := node.CopyToMemory(nil)
	//require.NoError(T, err)
	//
	//memnode := copied.(*MemoryNode)
	//for i := range memnode.keypaths {
	//    require.Equal(T, expectedKeypaths[i], memnode.keypaths[i])
	//}
}

func TestDBTree_CopyToMemory_AtKeypath(T *testing.T) {
	T.Parallel()

	i := rand.Int()
	tree, err := NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(T, err)
	defer tree.DeleteDB()

	v := types.RandomID()

	err = tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(nil, nil, fixture1.input)
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)

	err = tree.View(v, func(node *DBNode) error {
		copied, err := node.AtKeypath(Keypath("flox"), nil).CopyToMemory(nil, nil)
		require.NoError(T, err)

		expected := takeFixtureOutputsWithPrefix(Keypath("flox"), fixture1.output...)
		expected = removeFixtureOutputPrefixes(Keypath("flox"), expected...)
		memnode := copied.(*MemoryNode)
		require.Equal(T, len(expected), len(memnode.keypaths))

		for i := range memnode.keypaths {
			require.Equal(T, expected[i].keypath, memnode.keypaths[i])
		}
		return nil
	})
	require.NoError(T, err)
}

func setRangeSlice_setup(T *testing.T) (*DBTree, types.ID, func()) {
	i := rand.Int()
	tree, err := NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(T, err)

	v := types.RandomID()

	err = tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/bar/baz"), nil, uint64(123))
		require.NoError(T, err)

		err = tx.Set(Keypath("foo/slice"), nil, []interface{}{testVal1, testVal2, testVal3, testVal4})
		require.NoError(T, err)

		return nil
	})
	require.NoError(T, err)

	val, exists, err := tree.Value(v, nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, val, M{
		"foo": M{
			"bar": M{
				"baz": uint64(123),
			},
			"slice": []interface{}{testVal1, testVal2, testVal3, testVal4},
		},
	})

	return tree, v, func() {
		err := tree.DeleteDB()
		require.NoError(T, err)
	}
}

func TestDBTree_SetRangeString(T *testing.T) {
	T.Parallel()

	i := rand.Int()
	tree, err := NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(T, err)
	defer tree.DeleteDB()
	v := types.RandomID()

	err = tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/string"), nil, "abcdefgh")
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)

	str, exists, err := tree.Value(v, Keypath("foo/string"), nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, "abcdefgh", str)

	err = tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/string"), &Range{3, 6}, "xx")
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)

	str, exists, err = tree.Value(v, Keypath("foo/string"), nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, "abcxxgh", str)
}

func TestDBTree_SetRangeSlice_Start_Grow(T *testing.T) {
	T.Parallel()

	tree, v, cleanup := setRangeSlice_setup(T)
	defer cleanup()

	err := tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/slice"), &Range{0, 2}, []interface{}{testVal5, testVal6, testVal7, testVal8})
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)

	val, exists, err := tree.Value(v, nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, M{
		"foo": M{
			"bar": M{
				"baz": uint64(123),
			},
			"slice": []interface{}{testVal5, testVal6, testVal7, testVal8, testVal3, testVal4},
		},
	}, val)
}

func TestDBTree_SetRangeSlice_Start_Same(T *testing.T) {
	T.Parallel()

	tree, v, cleanup := setRangeSlice_setup(T)
	defer cleanup()

	err := tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/slice"), &Range{0, 2}, []interface{}{testVal5, testVal6})
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)
	//debugPrint(T, tree)

	val, exists, err := tree.Value(v, nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, M{
		"foo": M{
			"bar": M{
				"baz": uint64(123),
			},
			"slice": []interface{}{testVal5, testVal6, testVal3, testVal4},
		},
	}, val)
}

func TestDBTree_SetRangeSlice_Start_Shrink(T *testing.T) {
	T.Parallel()

	tree, v, cleanup := setRangeSlice_setup(T)
	defer cleanup()

	err := tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/slice"), &Range{0, 2}, []interface{}{testVal5})
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)
	//debugPrint(T, tree)

	val, exists, err := tree.Value(v, nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, M{
		"foo": M{
			"bar": M{
				"baz": uint64(123),
			},
			"slice": []interface{}{testVal5, testVal3, testVal4},
		},
	}, val)
}

func TestDBTree_SetRangeSlice_Middle_Grow(T *testing.T) {
	T.Parallel()

	tree, v, cleanup := setRangeSlice_setup(T)
	defer cleanup()
	//debugPrint(T, tree)

	err := tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/slice"), &Range{1, 3}, []interface{}{testVal5, testVal6, testVal7, testVal8})
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)
	//debugPrint(T, tree)

	val, exists, err := tree.Value(v, nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, M{
		"foo": M{
			"bar": M{
				"baz": uint64(123),
			},
			"slice": []interface{}{testVal1, testVal5, testVal6, testVal7, testVal8, testVal4},
		},
	}, val)
}

func TestDBTree_SetRangeSlice_Middle_Same(T *testing.T) {
	T.Parallel()

	tree, v, cleanup := setRangeSlice_setup(T)
	defer cleanup()

	err := tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/slice"), &Range{1, 3}, []interface{}{testVal5, testVal6})
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)
	//debugPrint(T, tree)

	val, exists, err := tree.Value(v, nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, M{
		"foo": M{
			"bar": M{
				"baz": uint64(123),
			},
			"slice": []interface{}{testVal1, testVal5, testVal6, testVal4},
		},
	}, val)
}

func TestDBTree_SetRangeSlice_Middle_Shrink(T *testing.T) {
	T.Parallel()

	tree, v, cleanup := setRangeSlice_setup(T)
	defer cleanup()

	//debugPrint(T, tree)

	err := tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/slice"), &Range{1, 3}, []interface{}{testVal5})
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)
	//debugPrint(T, tree)

	val, exists, err := tree.Value(v, nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, M{
		"foo": M{
			"bar": M{
				"baz": uint64(123),
			},
			"slice": []interface{}{testVal1, testVal5, testVal4},
		},
	}, val)
}

func TestDBTree_SetRangeSlice_End_Grow(T *testing.T) {
	T.Parallel()

	tree, v, cleanup := setRangeSlice_setup(T)
	defer cleanup()

	err := tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/slice"), &Range{2, 4}, []interface{}{testVal5, testVal6, testVal7, testVal8})
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)
	//debugPrint(T, tree)

	val, exists, err := tree.Value(v, nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, M{
		"foo": M{
			"bar": M{
				"baz": uint64(123),
			},
			"slice": []interface{}{testVal1, testVal2, testVal5, testVal6, testVal7, testVal8},
		},
	}, val)
}

func TestDBTree_SetRangeSlice_End_Same(T *testing.T) {
	T.Parallel()

	tree, v, cleanup := setRangeSlice_setup(T)
	defer cleanup()

	err := tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/slice"), &Range{2, 4}, []interface{}{testVal5, testVal6})
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)
	//debugPrint(T, tree)

	val, exists, err := tree.Value(v, nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, M{
		"foo": M{
			"bar": M{
				"baz": uint64(123),
			},
			"slice": []interface{}{testVal1, testVal2, testVal5, testVal6},
		},
	}, val)
}

func TestDBTree_SetRangeSlice_End_Shrink(T *testing.T) {
	T.Parallel()

	tree, v, cleanup := setRangeSlice_setup(T)
	defer cleanup()

	err := tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/slice"), &Range{1, 4}, []interface{}{testVal5})
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)
	//debugPrint(T, tree)

	val, exists, err := tree.Value(v, nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, M{
		"foo": M{
			"bar": M{
				"baz": uint64(123),
			},
			"slice": []interface{}{testVal1, testVal5},
		},
	}, val)
}

func TestDBTree_SetRangeSlice_End_Append(T *testing.T) {
	T.Parallel()

	tree, v, cleanup := setRangeSlice_setup(T)
	defer cleanup()

	err := tree.Update(v, func(tx *DBNode) error {
		err := tx.Set(Keypath("foo/slice"), &Range{4, 4}, []interface{}{testVal5, testVal6, testVal7, testVal8})
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)
	//debugPrint(T, tree)

	val, exists, err := tree.Value(v, nil, nil)
	require.True(T, exists)
	require.NoError(T, err)
	require.Equal(T, M{
		"foo": M{
			"bar": M{
				"baz": uint64(123),
			},
			"slice": []interface{}{testVal1, testVal2, testVal3, testVal4, testVal5, testVal6, testVal7, testVal8},
		},
	}, val)
}

func TestDBTree_CopyVersion(T *testing.T) {
	T.Parallel()

	i := rand.Int()
	tree, err := NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(T, err)
	defer tree.DeleteDB()

	srcVersion := types.RandomID()
	dstVersion := types.RandomID()

	err = tree.Update(srcVersion, func(tx *DBNode) error {
		err := tx.Set(nil, nil, fixture1.input)
		require.NoError(T, err)
		return nil
	})
	require.NoError(T, err)

	err = tree.CopyVersion(dstVersion, srcVersion)
	require.NoError(T, err)

	srcVal, exists, err := tree.RootNode(srcVersion).Value(nil, nil)
	require.NoError(T, err)
	require.True(T, exists)
	require.Equal(T, srcVal, fixture1.input)

	dstVal, exists, err := tree.RootNode(dstVersion).Value(nil, nil)
	require.NoError(T, err)
	require.True(T, exists)
	require.Equal(T, dstVal, fixture1.input)

	var count int
	err = tree.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		iter := txn.NewIterator(opts)
		defer iter.Close()

		for iter.Rewind(); iter.Valid(); iter.Next() {
			count++
		}
		return nil
	})
	require.NoError(T, err)
	require.Equal(T, len(fixture1.output)*2, count)
}

//func TestDBTree_encodeGoValue(T *testing.T) {
//    T.Parallel()
//
//    cases := []struct {
//        input    interface{}
//        expected []byte
//    }{
//        {"asdf", []byte("vsasdf")},
//        {float64(321.23), []byte("vf")},
//    }
//
//    encodeGoValue()
//}

//func debugPrint(T *testing.T, tree *DBNode) {
//    keypaths, values, err := tree.Contents(nil, nil)
//    require.NoError(T, err)
//
//    fmt.Println("KEYPATHS:")
//    for i, kp := range keypaths {
//        fmt.Println("  -", kp, ":", values[i])
//    }
//
//    v, _, err := tree.Value(nil, nil)
//    require.NoError(T, err)
//
//    fmt.Println(prettyJSON(v))
//}

func prettyJSON(x interface{}) string {
	j, _ := json.MarshalIndent(x, "", "    ")
	return string(j)
}
