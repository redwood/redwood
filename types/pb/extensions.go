package pb

import (
	"redwood.dev/errors"
)

var ErrInvalidRange = errors.New("invalid range")

// func (rng *Range) Copy() *Range {
// 	if rng == nil {
// 		return nil
// 	}
// 	return &Range{rng.Start, rng.End, rng.Reverse}
// }

// func (rng Range) Valid() bool {
// 	if rng.Reverse {
// 		return rng.End <= rng.Start
// 	}
// 	return rng.End >= rng.Start
// }

func (rng Range) Length() uint64 {
	start, end := sort(rng.Start, rng.End)
	return end - start
}

// func (rng Range) ValidForLength(length uint64) bool {
// 	if !rng.Valid() {
// 		return false
// 	}
// 	if rng.Reverse {
// 		return rng.Start <= length
// 	}
// 	return rng.End <= length
// }

func (rng Range) NormalizedForLength(length uint64) Range {
	smaller, larger := sort(min(rng.Start, length), min(rng.End, length))
	if rng.Reverse {
		return Range{length - larger, length - smaller, false}
	}
	return Range{smaller, larger, false}
}

func (rng Range) Intersection(other Range) (Range, bool) {
	if rng.Reverse || other.Reverse {
		panic("cannot take intersection with a reversed range")
	}
	if rng.End < other.Start || rng.Start > other.End {
		return Range{}, false
	}
	return Range{max(rng.Start, other.Start), min(rng.End, other.End), false}, true
}

func sort(a, b uint64) (uint64, uint64) {
	if a < b {
		return a, b
	}
	return b, a
}

func max(a, b uint64) uint64 {
	if a < b {
		return b
	}
	return a
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func (alg HashAlg) String() string {
	switch alg {
	case HashAlgUnknown:
		return "unknown"
	case SHA1:
		return "sha1"
	case SHA3:
		return "sha3"
	default:
		return "ERR:(bad value for HashAlg)"
	}
}

func (alg HashAlg) MarshalJSON() ([]byte, error) {
	return []byte(`"` + alg.String() + `"`), nil
}

func (alg *HashAlg) UnmarshalJSON(bs []byte) error {
	switch string(bs) {
	case `"unknown"`:
		*alg = HashAlgUnknown
	case `"sha1"`, `"SHA1"`:
		*alg = SHA1
	case `"sha3"`, `"SHA3"`:
		*alg = SHA3
	default:
		return errors.Errorf("ERR:(bad value for HashAlg: '%v')", string(bs))
	}
	return nil
}
