package pb

import math "math"

func (rng *Range) Copy() *Range {
	if rng == nil {
		return nil
	}
	return &Range{rng.Start, rng.End}
}

func (rng *Range) Valid() bool {
	if rng.End < rng.Start {
		return false
	}
	if rng.Start < 0 && rng.End > 0 {
		return false
	}
	return true
}

func (rng *Range) Length() uint64 {
	if IsNegativeZero(rng.Start) {
		if IsNegativeZero(rng.End) {
			return 0
		}
		return uint64(math.Abs(rng.Start - rng.End))
	}

	if rng.Start < 0 {
		return uint64(-(rng.Start - rng.End))
	}
	return uint64(rng.End - rng.Start)
}

func (rng *Range) ValidForLength(length uint64) bool {
	var start, end float64
	if IsNegativeZero(rng.Start) {
		start = float64(length)
	} else {
		start = rng.Start
	}

	if IsNegativeZero(rng.End) {
		end = float64(length)
	} else {
		end = rng.End
	}

	if start < 0 {
		return uint64(-start) <= length
	}
	return uint64(end) <= length
}

func (rng *Range) IndicesForLength(length uint64) (uint64, uint64) {
	var start, end float64
	if IsNegativeZero(rng.Start) {
		start = float64(length)
	} else {
		start = rng.Start
	}

	if IsNegativeZero(rng.End) {
		end = float64(length)
	} else {
		end = rng.End
	}

	if start < 0 {
		return uint64(float64(length) + start), uint64(float64(length) + end)
	}
	return uint64(start), uint64(end)
}

var NegativeZero = func(f float64) float64 { return -f }(0)

func IsNegativeZero(f float64) bool {
	return math.Signbit(f) && f == 0
}
