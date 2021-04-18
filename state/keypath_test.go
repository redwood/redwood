package state_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/state"
)

func TestKeypathShift(T *testing.T) {
	tests := []struct {
		input        state.Keypath
		expectedTop  state.Keypath
		expectedRest state.Keypath
	}{
		{state.Keypath("foo/bar/baz"), state.Keypath("foo"), state.Keypath("bar/baz")},
		{state.Keypath("foo"), state.Keypath("foo"), nil},
		{state.Keypath(""), state.Keypath(""), nil},
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
		input    state.Keypath
		extra    state.Keypath
		expected state.Keypath
	}{
		{state.Keypath("bar/baz"), state.Keypath("foo"), state.Keypath("foo/bar/baz")},
		{state.Keypath(""), state.Keypath("foo"), state.Keypath("foo")},
		{nil, state.Keypath("foo"), state.Keypath("foo")},
		{nil, nil, nil},
	}

	for _, test := range tests {
		out := test.input.Unshift(test.extra)
		require.Equal(T, test.expected, out)
	}
}

func TestKeypathPop(T *testing.T) {
	tests := []struct {
		input        state.Keypath
		expectedTop  state.Keypath
		expectedRest state.Keypath
	}{
		{state.Keypath("foo/bar/baz"), state.Keypath("baz"), state.Keypath("foo/bar")},
		{state.Keypath("foo"), state.Keypath("foo"), nil},
		{state.Keypath(""), state.Keypath(""), nil},
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
		input    state.Keypath
		extra    state.Keypath
		expected state.Keypath
	}{
		{state.Keypath("foo/bar"), state.Keypath("baz"), state.Keypath("foo/bar/baz")},
		{state.Keypath(""), state.Keypath("foo"), state.Keypath("foo")},
		{nil, state.Keypath("foo"), state.Keypath("foo")},
		{nil, nil, nil},
	}

	for _, test := range tests {
		out := test.input.Push(test.extra)
		require.Equal(T, test.expected, out)
	}
}

func TestKeypathPart(T *testing.T) {
	tests := []struct {
		input    state.Keypath
		idx      int
		expected state.Keypath
	}{
		{state.Keypath("foo/bar/baz/xyzzy"), 1, state.Keypath("bar")},
		{state.Keypath("foo/bar/baz/xyzzy"), 2, state.Keypath("baz")},
		{state.Keypath("foo/bar/baz/xyzzy"), -1, state.Keypath("xyzzy")},
		{state.Keypath("foo/slice").Push(state.EncodeSliceIndex(123)), 2, state.EncodeSliceIndex(123)},
		{state.Keypath("foo/slice").Push(state.EncodeSliceIndex(123)).Push(state.Keypath("xyzzy")), 3, state.Keypath("xyzzy")},
		{state.Keypath("foo"), -1, state.Keypath("foo")},
		{state.Keypath("foo"), 0, state.Keypath("foo")},
		{state.Keypath(""), 0, state.Keypath("")},
		{state.Keypath(""), 1, nil},
		{state.Keypath(""), -1, state.Keypath("")},
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
		input    state.Keypath
		expected []state.Keypath
	}{
		{state.Keypath("foo/bar"), []state.Keypath{state.Keypath("foo"), state.Keypath("bar")}},
		{state.Keypath("foo"), []state.Keypath{state.Keypath("foo")}},
		{state.Keypath(""), nil},
		{nil, nil},
	}

	for _, test := range tests {
		out := test.input.Parts()
		require.Equal(T, test.expected, out)
	}
}

func TestKeypathStartsWith(T *testing.T) {
	tests := []struct {
		input    state.Keypath
		prefix   state.Keypath
		expected bool
	}{
		{state.Keypath("fo"), state.Keypath("foo"), false},
		{state.Keypath("foo"), state.Keypath("foo"), true},
		{state.Keypath("foo/bar"), state.Keypath("foo"), true},
		{state.Keypath("foo/bar"), state.Keypath(nil), true},
		{state.Keypath("foo/bar"), state.Keypath("foo/bar"), true},
		{state.Keypath("foo/bar/baz"), state.Keypath("foo/bar"), true},
		{state.Keypath("foo/barx"), state.Keypath("foo/bar"), false},
		{state.Keypath("foox/bar"), state.Keypath("foo"), false},
		//{state.Keypath(""), nil},
		//{nil, nil},
	}

	for _, test := range tests {
		does := test.input.StartsWith(test.prefix)
		require.Equal(T, test.expected, does)
	}
}
