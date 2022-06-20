package pb_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"redwood.dev/types"
)

// func TestRange_Valid(t *testing.T) {
// 	t.Parallel()

// 	tests := []struct {
// 		name       string
// 		start, end uint64
// 		reverse    bool
// 		valid      bool
// 	}{
// 		{"[0:0]", 0, 0, false, true},
// 		{"[0:5]", 0, 5, false, true},
// 		{"[3:5]", 3, 5, false, true},
// 		{"[3:0]", 3, 0, false, false},
// 		{"[-0:-0]", 0, 0, true, true},
// 		{"[-7:-0]", 7, 0, true, true},
// 		{"[-7:-4]", 7, 4, true, true},
// 		{"[-4:-7]", 4, 7, true, false},
// 		{"[-0:-7]", 0, 7, true, false},
// 	}

// 	for _, test := range tests {
// 		test := test

// 		t.Run(test.name, func(t *testing.T) {
// 			t.Parallel()

// 			rng := &types.Range{test.start, test.end, test.reverse}
// 			require.Equal(t, test.valid, rng.Valid())
// 		})
// 	}
// }

func TestRange_Length(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		start, end uint64
		reverse    bool
		length     uint64
	}{
		{"[0:0]", 0, 0, false, 0},
		{"[0:5]", 0, 5, false, 5},
		{"[3:5]", 3, 5, false, 2},
		{"[3:0]", 3, 0, false, 3},
		{"[-0:-0]", 0, 0, true, 0},
		{"[-7:-0]", 7, 0, true, 7},
		{"[-7:-4]", 7, 4, true, 3},
		{"[-4:-7]", 4, 7, true, 3},
		{"[-0:-7]", 0, 7, true, 7},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			rng := &types.Range{test.start, test.end, test.reverse}
			require.Equal(t, test.length, rng.Length())
		})
	}
}

func TestRange_NormalizedForLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rng      types.Range
		length   uint64
		expected types.Range
	}{
		{"[0:0]", types.Range{0, 0, false}, 10, types.Range{0, 0, false}},
		{"[0:5]", types.Range{0, 5, false}, 10, types.Range{0, 5, false}},
		{"[3:5]", types.Range{3, 5, false}, 10, types.Range{3, 5, false}},
		{"[3:0]", types.Range{3, 0, false}, 10, types.Range{0, 3, false}},

		{"[-0:-0]", types.Range{0, 0, true}, 10, types.Range{10, 10, false}},
		{"[-7:-0]", types.Range{7, 0, true}, 10, types.Range{3, 10, false}},
		// {"[-7:-4]", types.Range{7, 4, false}, true, },
		// {"[-4:-7]", types.Range{4, 7, false}, true, },
		// {"[-0:-7]", types.Range{0, 7, false}, true, },
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			normalized := test.rng.NormalizedForLength(test.length)
			require.Equal(t, test.expected, normalized)
		})
	}
}

func TestRange_Intersection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		a, b         types.Range
		intersects   bool
		intersection types.Range
	}{
		{"[1:7] [3:9]", types.Range{1, 7, false}, types.Range{3, 9, false}, true, types.Range{3, 7, false}},
		{"[1:9] [1:9]", types.Range{1, 9, false}, types.Range{1, 9, false}, true, types.Range{1, 9, false}},
		{"[1:9] [3:6]", types.Range{1, 9, false}, types.Range{3, 6, false}, true, types.Range{3, 6, false}},
		{"[1:6] [3:8]", types.Range{1, 6, false}, types.Range{3, 8, false}, true, types.Range{3, 6, false}},
		{"[3:5] [7:9]", types.Range{3, 5, false}, types.Range{7, 9, false}, false, types.Range{}},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			intersection, intersects := test.a.Intersection(test.b)
			require.Equal(t, test.intersects, intersects)
			require.Equal(t, test.intersection, intersection)

			intersection, intersects = test.b.Intersection(test.a)
			require.Equal(t, test.intersects, intersects)
			require.Equal(t, test.intersection, intersection)
		})
	}
}
