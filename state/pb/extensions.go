package pb

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
	if rng.Start < 0 {
		return uint64(-(rng.Start - rng.End))
	}
	return uint64(rng.End - rng.Start)
}

func (rng *Range) ValidForLength(length uint64) bool {
	if rng.Start < 0 {
		return uint64(-rng.Start) <= length
	}
	return uint64(rng.End) <= length
}

func (rng *Range) IndicesForLength(length uint64) (uint64, uint64) {
	if rng.Start < 0 {
		return uint64(int64(length) + rng.Start), uint64(int64(length) + rng.End)
	}
	return uint64(rng.Start), uint64(rng.End)
}
