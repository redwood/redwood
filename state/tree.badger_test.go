package state_test

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"redwood.dev/internal/testutils"
	"redwood.dev/state"
	"redwood.dev/types"
)

func TestVersionedDBTree_Value_MapWithRange(t *testing.T) {
	tests := []struct {
		start, end int64
		expected   interface{}
	}{
		{0, 1, M{
			"asdf": S{"1234", float64(987.2), uint64(333)}},
		},
		{0, 2, M{
			"asdf": S{"1234", float64(987.2), uint64(333)},
			"flo":  float64(321),
		}},
		{1, 2, M{
			"flo": float64(321),
		}},
		{1, 3, M{
			"flo": float64(321),
			"flox": S{
				uint64(65),
				M{"yup": "yes", "hey": uint64(321)},
				"jkjkjkj",
			},
		}},
		{0, 5, M{
			"asdf": S{"1234", float64(987.2), uint64(333)},
			"flo":  float64(321),
			"flox": S{
				uint64(65),
				M{"yup": "yes", "hey": uint64(321)},
				"jkjkjkj",
			},
			"floxxx": "asdf123",
			"hello": M{
				"xyzzy": uint64(33),
			},
		}},
		{0, 0, M{}},
		{5, 5, state.ErrInvalidRange},
		{6, 6, state.ErrInvalidRange},
		{-2, 0, M{
			"floxxx": "asdf123",
			"hello": M{
				"xyzzy": uint64(33),
			},
		}},
	}

	rootKeypaths := []state.Keypath{state.Keypath(nil)}

	for _, rootKeypath := range rootKeypaths {
		for _, test := range tests {
			test := test
			rootKeypath := rootKeypath
			name := fmt.Sprintf("%v[%v:%v]", rootKeypath, test.start, test.end)

			t.Run(name, func(t *testing.T) {
				db := testutils.SetupVersionedDBTreeWithValue(t, rootKeypath, fixture1.input)
				defer db.DeleteDB()

				node := db.StateAtVersion(nil, false)

				val, exists, err := node.Value(rootKeypath, &state.Range{test.start, test.end})
				switch exp := test.expected.(type) {
				case error:
					require.True(t, errors.Cause(exp) == test.expected)
				default:
					require.NoError(t, err)
					require.True(t, exists)
					require.Equal(t, exp, val)
				}
			})
		}
	}
}

func TestVersionedDBTree_Value_SliceWithRange(t *testing.T) {
	tests := []struct {
		start, end int64
		expected   interface{}
	}{
		{0, 1, S{
			uint64(8383),
		}},
		{0, 2, S{
			uint64(8383),
			M{"9999": "hi", "vvvv": "yeah"},
		}},
		{1, 2, S{
			M{"9999": "hi", "vvvv": "yeah"},
		}},
		{1, 3, S{
			M{"9999": "hi", "vvvv": "yeah"},
			float64(321.23),
		}},
		{0, 3, S{
			uint64(8383),
			M{"9999": "hi", "vvvv": "yeah"},
			float64(321.23),
		}},
		{0, 0, S{}},
		{4, 4, state.ErrInvalidRange},
		{-2, 0, S{
			float64(321.23),
			"hello",
		}},
		{-2, -1, S{
			float64(321.23),
		}},
	}

	for _, test := range tests {
		test := test
		name := fmt.Sprintf("[%v : %v]", test.start, test.end)
		t.Run(name, func(t *testing.T) {
			db := testutils.SetupVersionedDBTreeWithValue(t, nil, fixture3.input)
			defer db.DeleteDB()

			node := db.StateAtVersion(nil, false)

			val, exists, err := node.Value(state.Keypath(nil), &state.Range{test.start, test.end})
			switch exp := test.expected.(type) {
			case error:
				require.True(t, errors.Cause(exp) == test.expected)
			default:
				require.NoError(t, err)
				require.True(t, exists)
				require.Equal(t, exp, val)
			}
		})
	}
}

func TestDBNode_Set_NoRange(t *testing.T) {
	t.Run("slice", func(t *testing.T) {
		db := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("data"), fixture1.input)
		defer db.DeleteDB()

		node := db.StateAtVersion(nil, true)

		err := node.Set(state.Keypath("data/flox"), nil, S{"a", "b", "c", "d"})
		require.NoError(t, err)

		err = node.Save()
		require.NoError(t, err)

		node = db.StateAtVersion(nil, false)
		defer node.Close()

		val, exists, err := node.Value(state.Keypath("data/flox"), nil)
		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, S{"a", "b", "c", "d"}, val)
	})

	t.Run("struct", func(t *testing.T) {
		type SomeStruct struct {
			Foo string `tree:"foo"`
			Bar uint64 `tree:"bar"`
		}
		type TestStruct struct {
			Asdf       []interface{}          `tree:"asdf"`
			Flo        float64                `tree:"flo"`
			Flox       []interface{}          `tree:"flox"`
			Floxx      string                 `tree:"floxx"`
			Hello      map[string]interface{} `tree:"hello"`
			SomeStruct SomeStruct             `tree:"someStruct"`
		}

		val := TestStruct{
			Asdf: S{"1234", float64(987.2), uint64(333)},
			Flo:  321,
			Flox: S{
				uint64(65),
				M{"yup": "yes", "hey": uint64(321)},
				"jkjkjkj",
			},
			Floxx: "asdf123",
			Hello: M{
				"xyzzy": uint64(33),
			},
			SomeStruct: SomeStruct{
				Foo: "fooooo",
				Bar: 54321,
			},
		}

		expected := M{
			"asdf": S{"1234", float64(987.2), uint64(333)},
			"flo":  float64(321),
			"flox": S{
				uint64(65),
				M{"yup": "yes", "hey": uint64(321)},
				"jkjkjkj",
			},
			"floxx": "asdf123",
			"hello": M{
				"xyzzy": uint64(33),
			},
			"someStruct": M{
				"foo": "fooooo",
				"bar": uint64(54321),
			},
		}

		db := testutils.SetupDBTree(t)
		defer db.DeleteDB()

		node := db.State(true)

		err := node.Set(state.Keypath("data"), nil, val)
		require.NoError(t, err)

		err = node.Save()
		require.NoError(t, err)

		node = db.State(false)
		got, exists, err := node.Value(state.Keypath("data"), nil)
		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, expected, got)
	})

	// t.Run("memory node", func(t *testing.T) {
	// 	db := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("data"), fixture1.input)
	// 	defer db.DeleteDB()

	// 	node := db.StateAtVersion(nil, true)

	// 	memNode := NewMemoryNode()

	// 	memNode.Set(nil, nil, M{
	// 		"foo": M{"one": uint64(1), "two": uint64(2)},
	// 		"bar": S{"hi", float64(123)},
	// 	})

	// 	err := node.Set(state.Keypath("data/flox"), nil, memNode)
	// 	require.NoError(t, err)

	// 	err = node.Save()
	// 	require.NoError(t, err)

	// 	node = db.StateAtVersion(nil, false)
	// 	state.DebugPrint(debugPrint, true, 0)
	// })

	// t.Run("db node inside memory node", func(t *testing.T) {
	// 	db := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("data"), fixture1.input)
	// 	defer db.DeleteDB()

	// 	node := db.StateAtVersion(nil, true)

	// 	memNode := NewMemoryNode()
	// 	innerDBNode := node.NodeAt(state.Keypath("data/flox"), nil)

	// 	memNode.Set(nil, nil, M{
	// 		"foo": innerDBNode,
	// 	})

	// 	memNode.DebugPrint(debugPrint, true, 0)

	// 	err := node.Set(state.Keypath("data/hello/xyzzy"), nil, memNode)
	// 	require.NoError(t, err)

	// 	err = node.Save()
	// 	require.NoError(t, err)

	// 	node = db.StateAtVersion(nil, false)
	// 	state.DebugPrint(debugPrint, true, 0)
	// })
}

func TestDBNode_Scan(t *testing.T) {
	t.Run("struct", func(t *testing.T) {
		type SomeStruct struct {
			Foo string `tree:"foo"`
			Bar uint64 `tree:"bar"`
		}
		type CustomBytes []byte
		type CustomByteArray [4]byte
		type TestStruct struct {
			Slice           []SomeStruct                        `tree:"slice"`
			Array           [3]SomeStruct                       `tree:"array"`
			Flo             float64                             `tree:"flo"`
			Flox            []interface{}                       `tree:"flox"`
			Floxx           string                              `tree:"floxx"`
			Bytes           []byte                              `tree:"bytes"`
			CustomBytes     CustomBytes                         `tree:"customBytes"`
			ByteArray       [3]byte                             `tree:"byteArray"`
			CustomByteArray CustomByteArray                     `tree:"customByteArray"`
			Map             map[string]interface{}              `tree:"map"`
			TypedMap        map[uint32]string                   `tree:"typedMap"`
			TypedMap2       map[CustomByteArray]CustomByteArray `tree:"typedMap2"`
			SomeStruct      SomeStruct                          `tree:"someStruct"`
		}

		expected := TestStruct{
			Slice: []SomeStruct{
				{"oof", 987},
				{"ofo", 654},
			},
			Array: [3]SomeStruct{
				{"one", 3},
				{"two", 2},
				{"three", 1},
			},
			Flo: 321,
			Flox: S{
				uint64(65),
				M{"yup": "yes", "hey": uint64(321)},
				"jkjkjkj",
			},
			Floxx:           "asdf123",
			Bytes:           []byte("the bytes"),
			CustomBytes:     CustomBytes("custom bytes"),
			ByteArray:       [3]byte{0x9, 0x5, 0x7},
			CustomByteArray: CustomByteArray{0x7, 0x5, 0x9, 0x8},
			Map: M{
				"xyzzy": uint64(33),
				"ewok":  true,
			},
			TypedMap: map[uint32]string{
				321: "zork",
				123: "kroz",
			},
			TypedMap2: map[CustomByteArray]CustomByteArray{
				CustomByteArray{6, 2, 4, 1}:     CustomByteArray{12, 14, 16, 18},
				CustomByteArray{61, 21, 41, 11}: CustomByteArray{16, 12, 14, 11},
			},
			SomeStruct: SomeStruct{
				Foo: "fooooo",
				Bar: 54321,
			},
		}

		fixture := M{
			"slice": S{
				M{"foo": "oof", "bar": uint64(987)},
				M{"foo": "ofo", "bar": uint64(654)},
			},
			"array": S{
				M{"foo": "one", "bar": uint64(3)},
				M{"foo": "two", "bar": uint64(2)},
				M{"foo": "three", "bar": uint64(1)},
			},
			"flo": float64(321),
			"flox": S{
				uint64(65),
				M{"yup": "yes", "hey": uint64(321)},
				"jkjkjkj",
			},
			"floxx":           "asdf123",
			"bytes":           []byte("the bytes"),
			"customBytes":     []byte("custom bytes"),
			"byteArray":       []byte{9, 5, 7},
			"customByteArray": []byte{7, 5, 9, 8},
			"map": M{
				"xyzzy": uint64(33),
				"ewok":  true,
			},
			"typedMap": map[uint32]string{
				321: "zork",
				123: "kroz",
			},
			"typedMap2": map[string][]byte{
				string([]byte{6, 2, 4, 1}):     []byte{12, 14, 16, 18},
				string([]byte{61, 21, 41, 11}): []byte{16, 12, 14, 11},
			},
			"someStruct": M{
				"foo": "fooooo",
				"bar": uint64(54321),
			},
		}

		db := testutils.SetupDBTreeWithValue(t, state.Keypath("data"), fixture)
		defer db.DeleteDB()

		node := db.State(false)
		defer node.Close()

		var got TestStruct
		err := node.NodeAt(state.Keypath("data"), nil).Scan(&got)
		require.NoError(t, err)
		require.Equal(t, expected, got)
	})
}

func TestVersionedDBTree_Set_Range_String(t *testing.T) {
	db := testutils.SetupVersionedDBTree(t)
	defer db.DeleteDB()
	v := types.RandomID()

	err := update(db, &v, func(tx *state.DBNode) error {
		err := tx.Set(state.Keypath("foo/string"), nil, "abcdefgh")
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	node := db.StateAtVersion(&v, false)
	defer node.Close()

	str, exists, err := node.Value(state.Keypath("foo/string"), nil)
	require.True(t, exists)
	require.NoError(t, err)
	require.Equal(t, "abcdefgh", str)
	node.Close()

	err = update(db, &v, func(tx *state.DBNode) error {
		err := tx.Set(state.Keypath("foo/string"), &state.Range{3, 6}, "xx")
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	node = db.StateAtVersion(&v, false)
	defer node.Close()

	str, exists, err = node.Value(state.Keypath("foo/string"), nil)
	require.True(t, exists)
	require.NoError(t, err)
	require.Equal(t, "abcxxgh", str)
}

func TestVersionedDBTree_Set_Range_Slice(t *testing.T) {

	tests := []struct {
		name          string
		setKeypath    state.Keypath
		setRange      *state.Range
		setVals       []interface{}
		expectedSlice []interface{}
	}{
		{"start grow", state.Keypath("foo/slice"), &state.Range{0, 2}, S{testVal5, testVal6, testVal7, testVal8},
			S{testVal5, testVal6, testVal7, testVal8, testVal3, testVal4}},
		{"start same", state.Keypath("foo/slice"), &state.Range{0, 2}, S{testVal5, testVal6},
			S{testVal5, testVal6, testVal3, testVal4}},
		{"start shrink", state.Keypath("foo/slice"), &state.Range{0, 2}, S{testVal5},
			S{testVal5, testVal3, testVal4}},
		{"middle grow", state.Keypath("foo/slice"), &state.Range{1, 3}, S{testVal5, testVal6, testVal7, testVal8},
			S{testVal1, testVal5, testVal6, testVal7, testVal8, testVal4}},
		{"middle same", state.Keypath("foo/slice"), &state.Range{1, 3}, S{testVal5, testVal6},
			S{testVal1, testVal5, testVal6, testVal4}},
		{"middle shrink", state.Keypath("foo/slice"), &state.Range{1, 3}, S{testVal5},
			S{testVal1, testVal5, testVal4}},
		{"end grow", state.Keypath("foo/slice"), &state.Range{2, 4}, S{testVal5, testVal6, testVal7, testVal8},
			S{testVal1, testVal2, testVal5, testVal6, testVal7, testVal8}},
		{"end same", state.Keypath("foo/slice"), &state.Range{2, 4}, S{testVal5, testVal6},
			S{testVal1, testVal2, testVal5, testVal6}},
		{"end shrink", state.Keypath("foo/slice"), &state.Range{1, 4}, S{testVal5},
			S{testVal1, testVal5}},
		{"end append", state.Keypath("foo/slice"), &state.Range{4, 4}, S{testVal5, testVal6, testVal7, testVal8},
			S{testVal1, testVal2, testVal3, testVal4, testVal5, testVal6, testVal7, testVal8}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			db := testutils.SetupVersionedDBTreeWithValue(t, nil, M{
				"foo": M{
					"bar":   M{"baz": uint64(123)},
					"slice": S{testVal1, testVal2, testVal3, testVal4},
				},
			})
			defer db.DeleteDB()

			node := db.StateAtVersion(nil, true)
			defer node.Close()

			err := node.Set(test.setKeypath, test.setRange, test.setVals)
			require.NoError(t, err)
			err = node.Save()
			require.NoError(t, err)

			node = db.StateAtVersion(nil, false)
			defer node.Close()

			val, exists, err := node.Value(nil, nil)
			require.True(t, exists)
			require.NoError(t, err)
			require.Equal(t, M{
				"foo": M{
					"bar":   M{"baz": uint64(123)},
					"slice": test.expectedSlice,
				},
			}, val)
		})
	}
}

func TestDBNode_Delete_NoRange(t *testing.T) {
	t.Run("slice", func(t *testing.T) {
		db := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("data"), fixture1.input)
		defer db.DeleteDB()

		node := db.StateAtVersion(nil, true)

		err := node.Delete(state.Keypath("data/flox"), nil)
		require.NoError(t, err)

		err = node.Save()
		require.NoError(t, err)

		node = db.StateAtVersion(nil, false)

		expected := append(
			makeSetKeypathFixtureOutputs(state.Keypath("data")),
			prefixFixtureOutputs(state.Keypath("data"), fixture1.output)...,
		)
		expected = removeFixtureOutputsWithPrefix(state.Keypath("data/flox"), expected...)

		iter := node.Iterator(nil, false, 0)
		defer iter.Close()

		i := 0
		for iter.Rewind(); iter.Valid(); iter.Next() {
			require.Equal(t, expected[i].keypath, iter.Node().Keypath())
			i++
		}
	})
}

func TestVersionedDBTree_CopyToMemory(t *testing.T) {
	tests := []struct {
		name    string
		keypath state.Keypath
	}{
		{"root value", state.Keypath(nil)},
		{"value", state.Keypath("flo")},
		{"slice", state.Keypath("flox")},
		{"map", state.Keypath("flox").PushIndex(1)},
	}

	t.Run("after .NodeAt", func(t *testing.T) {
		for _, test := range tests {
			test := test
			t.Run(test.name, func(t *testing.T) {
				db := testutils.SetupVersionedDBTreeWithValue(t, nil, fixture1.input)
				defer db.DeleteDB()

				node := db.StateAtVersion(nil, false)
				defer node.Close()

				copied, err := node.NodeAt(test.keypath, nil).CopyToMemory(nil, nil)
				require.NoError(t, err)

				expected := filterFixtureOutputsWithPrefix(test.keypath, fixture1.output...)
				expected = removeFixtureOutputPrefixes(test.keypath, expected...)

				memnode := copied.(*state.MemoryNode)
				require.Equal(t, len(expected), len(memnode.Keypaths()))
				for i := range memnode.Keypaths() {
					require.Equal(t, expected[i].keypath, memnode.Keypaths()[i])
				}
			})
		}
	})

	t.Run("without .NodeAt", func(t *testing.T) {
		for _, test := range tests {
			test := test
			t.Run(test.name, func(t *testing.T) {
				db := testutils.SetupVersionedDBTreeWithValue(t, nil, fixture1.input)
				defer db.DeleteDB()

				node := db.StateAtVersion(nil, false)
				defer node.Close()

				copied, err := node.CopyToMemory(test.keypath, nil)
				require.NoError(t, err)

				expected := filterFixtureOutputsWithPrefix(test.keypath, fixture1.output...)
				expected = removeFixtureOutputPrefixes(test.keypath, expected...)

				memnode := copied.(*state.MemoryNode)
				require.Equal(t, len(expected), len(memnode.Keypaths()))
				for i := range memnode.Keypaths() {
					require.Equal(t, expected[i].keypath, memnode.Keypaths()[i])
				}
			})
		}
	})

}

func TestDBNode_Iterator(t *testing.T) {
	tests := []struct {
		name        string
		setKeypath  state.Keypath
		iterKeypath state.Keypath
		fixture     fixture
	}{
		{"root set, root iter, map value", state.Keypath(nil), state.Keypath(nil), fixture1},
		{"root set, root iter, map value 2", state.Keypath(nil), state.Keypath(nil), fixture2},
		{"root set, root iter, float value", state.Keypath(nil), state.Keypath(nil), fixture5},
		{"root set, root iter, string value", state.Keypath(nil), state.Keypath(nil), fixture6},
		{"root set, root iter, bool value", state.Keypath(nil), state.Keypath(nil), fixture7},

		{"non-root set, root iter, map value", state.Keypath("foo/bar"), state.Keypath(nil), fixture1},
		{"non-root set, root iter, map value 2", state.Keypath("foo/bar"), state.Keypath(nil), fixture2},
		{"non-root set, root iter, float value", state.Keypath("foo/bar"), state.Keypath(nil), fixture5},
		{"non-root set, root iter, string value", state.Keypath("foo/bar"), state.Keypath(nil), fixture6},
		{"non-root set, root iter, bool value", state.Keypath("foo/bar"), state.Keypath(nil), fixture7},

		{"root set, non-root iter, map value", state.Keypath(nil), state.Keypath("flox"), fixture1},
		{"root set, non-root iter, map value 2", state.Keypath(nil), state.Keypath("flox"), fixture2},
		{"root set, non-root iter, float value", state.Keypath(nil), state.Keypath("flox"), fixture5},
		{"root set, non-root iter, string value", state.Keypath(nil), state.Keypath("flox"), fixture6},
		{"root set, non-root iter, bool value", state.Keypath(nil), state.Keypath("flox"), fixture7},

		{"non-root set, non-root iter, map value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture1},
		{"non-root set, non-root iter, map value 2", state.Keypath("foo/bar"), state.Keypath("flox"), fixture2},
		{"non-root set, non-root iter, float value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture5},
		{"non-root set, non-root iter, string value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture6},
		{"non-root set, non-root iter, bool value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			db := testutils.SetupVersionedDBTreeWithValue(t, test.setKeypath, test.fixture.input)
			defer db.DeleteDB()

			node := db.StateAtVersion(nil, false)

			setKeypathOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			valueOutputs := prefixFixtureOutputs(test.setKeypath, test.fixture.output)
			expected := append(setKeypathOutputs, valueOutputs...)
			expected = filterFixtureOutputsWithPrefix(test.iterKeypath, expected...)

			iter := node.Iterator(test.iterKeypath, false, 0)
			defer iter.Close()
			var i int
			for iter.Rewind(); iter.Valid(); iter.Next() {
				node := iter.Node()
				require.Equal(t, expected[i].keypath, node.Keypath())
				i++
			}
			require.Equal(t, len(expected), i)

		})
	}
}

func TestDBNode_ReusableIterator(t *testing.T) {
	val := M{
		"aaa": uint64(123),
		"bbb": uint64(123),
		"ccc": M{
			"111": M{
				"a": uint64(1),
				"b": uint64(1),
				"c": uint64(1),
			},
		},
		"ddd": uint64(123),
		"eee": uint64(123),
	}

	db := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("foo"), val)
	defer db.DeleteDB()

	node := db.StateAtVersion(nil, true)
	iter := node.Iterator(state.Keypath("foo"), false, 0)
	defer iter.Close()

	iter.Rewind()
	require.True(t, iter.Valid())
	require.Equal(t, state.Keypath("foo"), iter.Node().Keypath())

	iter.Next()
	require.True(t, iter.Valid())
	require.Equal(t, state.Keypath("foo/aaa"), iter.Node().Keypath())

	iter.Next()
	require.True(t, iter.Valid())
	require.Equal(t, state.Keypath("foo/bbb"), iter.Node().Keypath())

	iter.Next()
	require.True(t, iter.Valid())
	require.Equal(t, state.Keypath("foo/ccc"), iter.Node().Keypath())

	{
		reusableIter := iter.Node().Iterator(state.Keypath("111"), true, 10)
		require.IsType(t, &state.ReusableIterator{}, reusableIter)

		reusableIter.Rewind()
		require.True(t, reusableIter.Valid())
		require.Equal(t, state.Keypath("foo/ccc/111"), reusableIter.Node().Keypath())

		reusableIter.Next()
		require.True(t, reusableIter.Valid())
		require.Equal(t, state.Keypath("foo/ccc/111/a"), reusableIter.Node().Keypath())

		reusableIter.Next()
		require.True(t, reusableIter.Valid())
		require.Equal(t, state.Keypath("foo/ccc/111/b"), reusableIter.Node().Keypath())

		reusableIter.Next()
		require.True(t, reusableIter.Valid())
		require.Equal(t, state.Keypath("foo/ccc/111/c"), reusableIter.Node().Keypath())

		reusableIter.Next()
		require.False(t, reusableIter.Valid())

		require.True(t, iter.Valid())
		require.Equal(t, state.Keypath("foo/ccc"), iter.Node().Keypath())

		reusableIter.Close()

		require.Equal(t, []byte("foo/ccc"), iter.(*state.DBIterator).BadgerIter().Item().Key()[33:])

		iter.Next()
		require.True(t, iter.Valid())
		require.Equal(t, state.Keypath("foo/ccc/111"), iter.Node().Keypath())
	}
}

func TestDBNode_ChildIterator(t *testing.T) {
	tests := []struct {
		name        string
		setKeypath  state.Keypath
		iterKeypath state.Keypath
		fixture     fixture
	}{
		{"root set, root iter, map value", state.Keypath(nil), state.Keypath(nil), fixture1},
		{"root set, root iter, map value 2", state.Keypath(nil), state.Keypath(nil), fixture2},
		{"root set, root iter, float value", state.Keypath(nil), state.Keypath(nil), fixture5},
		{"root set, root iter, string value", state.Keypath(nil), state.Keypath(nil), fixture6},
		{"root set, root iter, bool value", state.Keypath(nil), state.Keypath(nil), fixture7},

		{"non-root set, root iter, map value", state.Keypath("foo/bar"), state.Keypath(nil), fixture1},
		{"non-root set, root iter, map value 2", state.Keypath("foo/bar"), state.Keypath(nil), fixture2},
		{"non-root set, root iter, float value", state.Keypath("foo/bar"), state.Keypath(nil), fixture5},
		{"non-root set, root iter, string value", state.Keypath("foo/bar"), state.Keypath(nil), fixture6},
		{"non-root set, root iter, bool value", state.Keypath("foo/bar"), state.Keypath(nil), fixture7},

		{"root set, non-root iter, map value", state.Keypath(nil), state.Keypath("flox"), fixture1},
		{"root set, non-root iter, map value 2", state.Keypath(nil), state.Keypath("flox"), fixture2},
		{"root set, non-root iter, float value", state.Keypath(nil), state.Keypath("flox"), fixture5},
		{"root set, non-root iter, string value", state.Keypath(nil), state.Keypath("flox"), fixture6},
		{"root set, non-root iter, bool value", state.Keypath(nil), state.Keypath("flox"), fixture7},

		{"non-root set, non-root iter, map value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture1},
		{"non-root set, non-root iter, map value 2", state.Keypath("foo/bar"), state.Keypath("flox"), fixture2},
		{"non-root set, non-root iter, float value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture5},
		{"non-root set, non-root iter, string value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture6},
		{"non-root set, non-root iter, bool value", state.Keypath("foo/bar"), state.Keypath("flox"), fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			db := testutils.SetupVersionedDBTreeWithValue(t, test.setKeypath, test.fixture.input)
			defer db.DeleteDB()

			node := db.StateAtVersion(nil, false)

			prefixOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			valueOutputs := combineFixtureOutputs(test.setKeypath, test.fixture)
			expected := append(prefixOutputs, valueOutputs...)
			expected = filterFixtureOutputsToDirectDescendantsOf(test.iterKeypath, expected...)

			iter := node.ChildIterator(test.iterKeypath, false, 0)
			defer iter.Close()
			var i int
			for iter.Rewind(); iter.Valid(); iter.Next() {
				node := iter.Node()
				require.Equal(t, expected[i].keypath, node.Keypath())
				i++
			}
			require.Equal(t, len(expected), i)

		})
	}
}

func TestDBNode_ReusableChildIterator(t *testing.T) {
	val := M{
		"aaa": uint64(123),
		"bbb": uint64(123),
		"ccc": M{
			"111": M{
				"a": uint64(1),
				"b": uint64(1),
				"c": uint64(1),
			},
		},
		"ddd": uint64(123),
		"eee": uint64(123),
	}

	db := testutils.SetupVersionedDBTreeWithValue(t, state.Keypath("foo"), val)
	defer db.DeleteDB()

	node := db.StateAtVersion(nil, true)
	iter := node.ChildIterator(state.Keypath("foo"), false, 0)
	defer iter.Close()

	iter.Rewind()
	require.True(t, iter.Valid())
	require.Equal(t, state.Keypath("foo/aaa"), iter.Node().Keypath())

	iter.Next()
	require.True(t, iter.Valid())
	require.Equal(t, state.Keypath("foo/bbb"), iter.Node().Keypath())

	iter.Next()
	require.True(t, iter.Valid())
	require.Equal(t, state.Keypath("foo/ccc"), iter.Node().Keypath())

	{
		reusableIter := iter.Node().Iterator(state.Keypath("111"), true, 10)
		require.IsType(t, &state.ReusableIterator{}, reusableIter)

		reusableIter.Rewind()
		require.True(t, reusableIter.Valid())
		require.Equal(t, state.Keypath("foo/ccc/111"), reusableIter.Node().Keypath())

		reusableIter.Next()
		require.True(t, reusableIter.Valid())
		require.Equal(t, state.Keypath("foo/ccc/111/a"), reusableIter.Node().Keypath())

		reusableIter.Next()
		require.True(t, reusableIter.Valid())
		require.Equal(t, state.Keypath("foo/ccc/111/b"), reusableIter.Node().Keypath())

		reusableIter.Next()
		require.True(t, reusableIter.Valid())
		require.Equal(t, state.Keypath("foo/ccc/111/c"), reusableIter.Node().Keypath())

		reusableIter.Next()
		require.False(t, reusableIter.Valid())

		require.True(t, iter.Valid())
		require.Equal(t, state.Keypath("foo/ccc"), iter.Node().Keypath())

		reusableIter.Close()

		// require.Equal(t, []byte("foo/ccc"), iter.(*dbChildIterator).iter.Item().Key()[33:])

		iter.Next()
		require.True(t, iter.Valid())
		require.Equal(t, state.Keypath("foo/ddd"), iter.Node().Keypath())
	}
}

func TestDBNode_DepthFirstIterator(t *testing.T) {
	tests := []struct {
		name        string
		setKeypath  state.Keypath
		iterKeypath state.Keypath
		fixture     fixture
	}{
		{"root set, root iter, map value", state.Keypath(nil), state.Keypath(nil), fixture1},
		{"root set, root iter, map value 2", state.Keypath(nil), state.Keypath(nil), fixture2},
		{"root set, root iter, float value", state.Keypath(nil), state.Keypath(nil), fixture5},
		{"root set, root iter, string value", state.Keypath(nil), state.Keypath(nil), fixture6},
		{"root set, root iter, bool value", state.Keypath(nil), state.Keypath(nil), fixture7},

		{"non-root set, root iter, map value", state.Keypath("foo/bar"), state.Keypath(nil), fixture1},
		{"non-root set, root iter, map value 2", state.Keypath("foo/bar"), state.Keypath(nil), fixture2},
		{"non-root set, root iter, float value", state.Keypath("foo/bar"), state.Keypath(nil), fixture5},
		{"non-root set, root iter, string value", state.Keypath("foo/bar"), state.Keypath(nil), fixture6},
		{"non-root set, root iter, bool value", state.Keypath("foo/bar"), state.Keypath(nil), fixture7},

		{"root set, non-root iter, map value", state.Keypath(nil), state.Keypath("flox"), fixture1},
		{"root set, non-root iter, map value 2", state.Keypath(nil), state.Keypath("eee"), fixture2},
		{"root set, non-root iter, float value", state.Keypath(nil), state.Keypath("flox"), fixture5},
		{"root set, non-root iter, string value", state.Keypath(nil), state.Keypath("flox"), fixture6},
		{"root set, non-root iter, bool value", state.Keypath(nil), state.Keypath("flox"), fixture7},

		{"non-root set, non-root iter, map value", state.Keypath("foo/bar"), state.Keypath("foo/bar/flox"), fixture1},
		{"non-root set, non-root iter, map value 2", state.Keypath("foo/bar"), state.Keypath("foo/bar/eee"), fixture2},
		{"non-root set, non-root iter, float value", state.Keypath("foo/bar"), state.Keypath("foo/bar"), fixture5},
		{"non-root set, non-root iter, string value", state.Keypath("foo/bar"), state.Keypath("foo/bar"), fixture6},
		{"non-root set, non-root iter, bool value", state.Keypath("foo/bar"), state.Keypath("foo/bar"), fixture7},
		{"non-root set, non-root iter, nonexistent value", state.Keypath("foo/bar"), state.Keypath("foo/bar/asdf"), fixture7},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			db := testutils.SetupVersionedDBTreeWithValue(t, test.setKeypath, test.fixture.input)
			defer db.DeleteDB()

			node := db.StateAtVersion(nil, false)

			prefixOutputs := makeSetKeypathFixtureOutputs(test.setKeypath)
			valueOutputs := combineFixtureOutputs(test.setKeypath, test.fixture)
			expected := append(prefixOutputs, valueOutputs...)
			expected = filterFixtureOutputsWithPrefix(test.iterKeypath, expected...)
			expected = reverseFixtureOutputs(expected...)

			iter := node.DepthFirstIterator(test.iterKeypath, false, 0)
			defer iter.Close()
			var i int
			for iter.Rewind(); iter.Valid(); iter.Next() {
				node := iter.Node()
				require.Equal(t, expected[i].keypath, node.Keypath())
				i++
			}
			require.Equal(t, len(expected), i)

		})
	}
}

func TestVersionedDBTree_CopyVersion(t *testing.T) {
	db := testutils.SetupVersionedDBTree(t)
	defer db.DeleteDB()

	srcVersion := types.RandomID()
	dstVersion := types.RandomID()

	err := update(db, &srcVersion, func(tx *state.DBNode) error {
		err := tx.Set(nil, nil, fixture1.input)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	err = db.CopyVersion(dstVersion, srcVersion)
	require.NoError(t, err)

	srcVal, exists, err := db.StateAtVersion(&srcVersion, false).Value(nil, nil)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, srcVal, fixture1.input)

	dstVal, exists, err := db.StateAtVersion(&dstVersion, false).Value(nil, nil)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, dstVal, fixture1.input)

	var count int
	err = db.BadgerDB().View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		iter := txn.NewIterator(opts)
		defer iter.Close()

		for iter.Rewind(); iter.Valid(); iter.Next() {
			count++
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, len(fixture1.output)*2, count)
}

// func TestVersionedDBTree_CopyToMemory(t *testing.T) {
//  t.Parallel()

//  i := rand.Int()
//  tree, err := NewVersionedDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
//  require.NoError(t, err)
//  defer state.DeleteDB()

//  err = update(tree, func(tx *state.DBNode) error {
//      _, err := tx.Set(nil, nil, testVal1)
//      require.NoError(t, err)
//      return nil
//  })
//  require.NoError(t, err)

//  expected := []struct {
//      keypath  state.Keypath
//      nodeType NodeType
//      val      interface{}
//  }{
//      {state.Keypath(""), NodeTypeMap, testVal1},
//      {state.Keypath("hello"), NodeTypeMap, testVal1["hello"]},
//      {state.Keypath("hello/xyzzy"), NodeTypeValue, testVal1["hello"].(M)["xyzzy"]},
//      {state.Keypath("flox"), NodeTypeSlice, testVal1["flox"]},
//      {state.Keypath("flox").PushIndex(0), NodeTypeValue, testVal1["flox"].(S)[0]},
//      {state.Keypath("flox").PushIndex(1), NodeTypeMap, testVal1["flox"].(S)[1]},
//      {state.Keypath("flox").PushIndex(1).Push(state.Keypath("yup")), NodeTypeValue, testVal1["flox"].(S)[1].(M)["yup"]},
//      {state.Keypath("flox").PushIndex(1).Push(state.Keypath("hey")), NodeTypeValue, testVal1["flox"].(S)[1].(M)["hey"]},
//      {state.Keypath("flox").PushIndex(2), NodeTypeValue, testVal1["flox"].(S)[2]},
//  }

//  expectedValues := map[string]interface{}{
//      "":                                   testVal1,
//      "hello":                              testVal1["hello"],
//      "hello/xyzzy":                        testVal1["hello"].(M)["xyzzy"],
//      "flox":                               testVal1["flox"],
//      string(state.Keypath("flox").PushIndex(0)): testVal1["flox"].(S)[0],
//      string(state.Keypath("flox").PushIndex(1)): testVal1["flox"].(S)[1],
//      string(state.Keypath("flox").PushIndex(1).Push(state.Keypath("yup"))): testVal1["flox"].(S)[1].(M)["yup"],
//      string(state.Keypath("flox").PushIndex(1).Push(state.Keypath("hey"))): testVal1["flox"].(S)[1].(M)["hey"],
//      string(state.Keypath("flox").PushIndex(2)):                      testVal1["flox"].(S)[2],
//  }

//  sort.Slice(expectedKeypaths, func(i, j int) bool { return bytes.Compare(expectedKeypaths[i], expectedKeypaths[j]) < 0 })

//  copied, err := node.CopyToMemory(nil)
//  require.NoError(t, err)

//  memnode := copied.(*MemoryNode)
//  for i := range memnode.Keypaths() {
//      require.Equal(t, expectedKeypaths[i], memnode.Keypaths()[i])
//  }
// }

//func TestVersionedDBTree_encodeGoValue(t *testing.T) {
//    t.Parallel()
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

//func debugPrint(t *testing.T, tree *state.DBNode) {
//    keypaths, values, err := state.Contents(nil, nil)
//    require.NoError(t, err)
//
//    fmt.Println("KEYPATHS:")
//    for i, kp := range keypaths {
//        fmt.Println("  -", kp, ":", values[i])
//    }
//
//    v, _, err := node.Value(nil, nil)
//    require.NoError(t, err)
//
//    fmt.Println(prettyJSON(v))
//}

func view(t *state.VersionedDBTree, v *types.ID, fn func(*state.DBNode) error) error {
	node := t.StateAtVersion(v, false)
	defer node.Close()
	return fn(node)
}

func update(t *state.VersionedDBTree, v *types.ID, fn func(*state.DBNode) error) error {
	node := t.StateAtVersion(v, true)
	defer node.Close()

	err := fn(node)
	if err != nil {
		return err
	}

	err = node.Save()
	if err != nil {
		return err
	}
	return nil
}
