package tree_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/tree"
)

func TestKeypathShift(T *testing.T) {
	tests := []struct {
		input        tree.Keypath
		expectedTop  tree.Keypath
		expectedRest tree.Keypath
	}{
		{tree.Keypath("foo/bar/baz"), tree.Keypath("foo"), tree.Keypath("bar/baz")},
		{tree.Keypath("foo"), tree.Keypath("foo"), nil},
		{tree.Keypath(""), tree.Keypath(""), nil},
		{nil, nil, nil},
	}

	for _, test := range tests {
		top, rest := test.input.Shift()
		require.Equal(T, test.expectedTop, top)
		require.Equal(T, test.expectedRest, rest)
	}
}

func TestKeypathUnshift(T *testing.T) {
	tests := []struct {
		input    tree.Keypath
		extra    tree.Keypath
		expected tree.Keypath
	}{
		{tree.Keypath("bar/baz"), tree.Keypath("foo"), tree.Keypath("foo/bar/baz")},
		{tree.Keypath(""), tree.Keypath("foo"), tree.Keypath("foo")},
		{nil, tree.Keypath("foo"), tree.Keypath("foo")},
		{nil, nil, nil},
	}

	for _, test := range tests {
		out := test.input.Unshift(test.extra)
		require.Equal(T, test.expected, out)
	}
}

func TestKeypathPop(T *testing.T) {
	tests := []struct {
		input        tree.Keypath
		expectedTop  tree.Keypath
		expectedRest tree.Keypath
	}{
		{tree.Keypath("foo/bar/baz"), tree.Keypath("baz"), tree.Keypath("foo/bar")},
		{tree.Keypath("foo"), tree.Keypath("foo"), nil},
		{tree.Keypath(""), tree.Keypath(""), nil},
		{nil, nil, nil},
	}

	for _, test := range tests {
		rest, top := test.input.Pop()
		require.Equal(T, test.expectedTop, top)
		require.Equal(T, test.expectedRest, rest)
	}
}

func TestKeypathPush(T *testing.T) {
	tests := []struct {
		input    tree.Keypath
		extra    tree.Keypath
		expected tree.Keypath
	}{
		{tree.Keypath("foo/bar"), tree.Keypath("baz"), tree.Keypath("foo/bar/baz")},
		{tree.Keypath(""), tree.Keypath("foo"), tree.Keypath("foo")},
		{nil, tree.Keypath("foo"), tree.Keypath("foo")},
		{nil, nil, nil},
	}

	for _, test := range tests {
		out := test.input.Push(test.extra)
		require.Equal(T, test.expected, out)
	}
}

func TestKeypathPart(T *testing.T) {
	tests := []struct {
		input    tree.Keypath
		idx      int
		expected tree.Keypath
	}{
		{tree.Keypath("foo/bar/baz/xyzzy"), 1, tree.Keypath("bar")},
		{tree.Keypath("foo/bar/baz/xyzzy"), 2, tree.Keypath("baz")},
		{tree.Keypath("foo/bar/baz/xyzzy"), -1, tree.Keypath("xyzzy")},
		{tree.Keypath("foo/slice").Push(tree.EncodeSliceIndex(123)), 2, tree.EncodeSliceIndex(123)},
		{tree.Keypath("foo/slice").Push(tree.EncodeSliceIndex(123)).Push(tree.Keypath("xyzzy")), 3, tree.Keypath("xyzzy")},
		{tree.Keypath("foo"), -1, tree.Keypath("foo")},
		{tree.Keypath("foo"), 0, tree.Keypath("foo")},
		{tree.Keypath(""), 0, tree.Keypath("")},
		{tree.Keypath(""), 1, nil},
		{tree.Keypath(""), -1, tree.Keypath("")},
		{nil, 0, nil},
		{nil, 1, nil},
		{nil, -1, nil},
	}

	for i, test := range tests {
		i := i
		test := test
		T.Run(fmt.Sprintf("%v", i), func(T *testing.T) {
			out := test.input.Part(test.idx)
			require.Equal(T, test.expected, out)
		})
	}
}

func TestKeypathParts(T *testing.T) {
	tests := []struct {
		input    tree.Keypath
		expected []tree.Keypath
	}{
		{tree.Keypath("foo/bar"), []tree.Keypath{tree.Keypath("foo"), tree.Keypath("bar")}},
		{tree.Keypath("foo"), []tree.Keypath{tree.Keypath("foo")}},
		{tree.Keypath(""), nil},
		{nil, nil},
	}

	for _, test := range tests {
		out := test.input.Parts()
		require.Equal(T, test.expected, out)
	}
}

func TestKeypathStartsWith(T *testing.T) {
	tests := []struct {
		input    tree.Keypath
		prefix   tree.Keypath
		expected bool
	}{
		{tree.Keypath("fo"), tree.Keypath("foo"), false},
		{tree.Keypath("foo"), tree.Keypath("foo"), true},
		{tree.Keypath("foo/bar"), tree.Keypath("foo"), true},
		{tree.Keypath("foo/bar"), tree.Keypath(nil), true},
		{tree.Keypath("foo/bar"), tree.Keypath("foo/bar"), true},
		{tree.Keypath("foo/bar/baz"), tree.Keypath("foo/bar"), true},
		{tree.Keypath("foo/barx"), tree.Keypath("foo/bar"), false},
		{tree.Keypath("foox/bar"), tree.Keypath("foo"), false},
		//{tree.Keypath(""), nil},
		//{nil, nil},
	}

	for _, test := range tests {
		does := test.input.StartsWith(test.prefix)
		require.Equal(T, test.expected, does)
	}
}
