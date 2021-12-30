package pb

import (
	"math"
)

func (rng *Range) Copy() *Range {
	if rng == nil {
		return nil
	}
	return &Range{rng.Start, rng.End, rng.Reverse}
}

func (rng *Range) Valid() bool {
	if rng.Reverse {
		return rng.End <= rng.Start
	}
	return rng.End >= rng.Start
}

func (rng *Range) Length() uint64 {
	return uint64(math.Abs(float64(rng.End) - float64(rng.Start)))
}

func (rng *Range) ValidForLength(length uint64) bool {
	if !rng.Valid() {
		return false
	}
	if rng.Reverse {
		return rng.Start <= length
	}
	return rng.End <= length
}

func (rng *Range) IndicesForLength(length uint64) (uint64, uint64) {
	if rng.Reverse {
		return length - rng.Start, length - rng.End
	}
	return rng.Start, rng.End
}
