package pb_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"redwood.dev/state/pb"
)

func TestRange_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		start, end uint64
		reverse    bool
		valid      bool
	}{
		{"[0:0]", 0, 0, false, true},
		{"[0:5]", 0, 5, false, true},
		{"[3:5]", 3, 5, false, true},
		{"[3:0]", 3, 0, false, false},
		{"[-0:-0]", 0, 0, true, true},
		{"[-7:-0]", 7, 0, true, true},
		{"[-7:-4]", 7, 4, true, true},
		{"[-4:-7]", 4, 7, true, false},
		{"[-0:-7]", 0, 7, true, false},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			rng := &pb.Range{test.start, test.end, test.reverse}
			require.Equal(t, test.valid, rng.Valid())
		})
	}
}

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

			rng := &pb.Range{test.start, test.end, test.reverse}
			require.Equal(t, test.length, rng.Length())
		})
	}
}
