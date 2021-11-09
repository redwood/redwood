package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/internal/testutils"
	"redwood.dev/state"
)

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
