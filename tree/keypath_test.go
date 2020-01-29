package tree

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeypathShift(T *testing.T) {
	tests := []struct {
		input        Keypath
		expectedTop  Keypath
		expectedRest Keypath
	}{
		{Keypath("foo/bar/baz"), Keypath("foo"), Keypath("bar/baz")},
		{Keypath("foo"), Keypath("foo"), nil},
		{Keypath(""), Keypath(""), nil},
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
		input    Keypath
		extra    Keypath
		expected Keypath
	}{
		{Keypath("bar/baz"), Keypath("foo"), Keypath("foo/bar/baz")},
		{Keypath(""), Keypath("foo"), Keypath("foo")},
		{nil, Keypath("foo"), Keypath("foo")},
		{nil, nil, nil},
	}

	for _, test := range tests {
		out := test.input.Unshift(test.extra)
		require.Equal(T, test.expected, out)
	}
}

func TestKeypathPop(T *testing.T) {
	tests := []struct {
		input        Keypath
		expectedTop  Keypath
		expectedRest Keypath
	}{
		{Keypath("foo/bar/baz"), Keypath("baz"), Keypath("foo/bar")},
		{Keypath("foo"), Keypath("foo"), nil},
		{Keypath(""), Keypath(""), nil},
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
		input    Keypath
		extra    Keypath
		expected Keypath
	}{
		{Keypath("foo/bar"), Keypath("baz"), Keypath("foo/bar/baz")},
		{Keypath(""), Keypath("foo"), Keypath("foo")},
		{nil, Keypath("foo"), Keypath("foo")},
		{nil, nil, nil},
	}

	for _, test := range tests {
		out := test.input.Push(test.extra)
		require.Equal(T, test.expected, out)
	}
}

func TestKeypathPart(T *testing.T) {
	tests := []struct {
		input    Keypath
		idx      int
		expected Keypath
	}{
		{Keypath("foo/bar/baz/xyzzy"), 1, Keypath("bar")},
		{Keypath("foo/bar/baz/xyzzy"), 2, Keypath("baz")},
		{Keypath("foo/bar/baz/xyzzy"), -1, Keypath("xyzzy")},
		{Keypath("foo/slice").Push(EncodeSliceIndex(123)), 2, EncodeSliceIndex(123)},
		{Keypath("foo/slice").Push(EncodeSliceIndex(123)).Push(Keypath("xyzzy")), 3, Keypath("xyzzy")},
		{Keypath("foo"), -1, Keypath("foo")},
		{Keypath("foo"), 0, Keypath("foo")},
		{Keypath(""), 0, Keypath("")},
		{Keypath(""), 1, nil},
		{Keypath(""), -1, Keypath("")},
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
		input    Keypath
		expected []Keypath
	}{
		{Keypath("foo/bar"), []Keypath{Keypath("foo"), Keypath("bar")}},
		{Keypath("foo"), []Keypath{Keypath("foo")}},
		{Keypath(""), nil},
		{nil, nil},
	}

	for _, test := range tests {
		out := test.input.Parts()
		require.Equal(T, test.expected, out)
	}
}

func TestKeypathStartsWith(T *testing.T) {
	tests := []struct {
		input    Keypath
		prefix   Keypath
		expected bool
	}{
		{Keypath("fo"), Keypath("foo"), false},
		{Keypath("foo"), Keypath("foo"), true},
		{Keypath("foo/bar"), Keypath("foo"), true},
		{Keypath("foo/bar"), Keypath(nil), true},
		{Keypath("foo/bar"), Keypath("foo/bar"), true},
		{Keypath("foo/bar/baz"), Keypath("foo/bar"), true},
		{Keypath("foo/barx"), Keypath("foo/bar"), false},
		{Keypath("foox/bar"), Keypath("foo"), false},
		//{Keypath(""), nil},
		//{nil, nil},
	}

	for _, test := range tests {
		does := test.input.StartsWith(test.prefix)
		require.Equal(T, test.expected, does)
	}
}
