package pb

import (
	"bytes"
	"strconv"

	"redwood.dev/errors"
	"redwood.dev/state"
)

var ErrBadPatch = errors.New("bad patch string")

func ParsePatch(s []byte) (Patch, error) {
	patch := Patch{}

	s = bytes.TrimSpace(s)

	for i := 0; i < len(s); {
		switch s[i] {
		case '.':
			key, err := parseDotKey(s[i:])
			if err != nil {
				return Patch{}, err
			}
			patch.Keypath = patch.Keypath.Push(key)
			i += len(key) + 1

		case '[':
			switch s[i+1] {
			case '"', '\'':
				key, err := parseBracketKey(s[i:])
				if err != nil {
					return Patch{}, err
				}
				patch.Keypath = patch.Keypath.Push(key)
				i += len(key) + 4

			case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				rng, idx, length, err := parseRangeOrIndex(s[i:])
				if err != nil {
					return Patch{}, err
				}
				if rng != nil {
					patch.Range = rng
				} else {
					patch.Keypath = patch.Keypath.PushIndex(idx)
				}
				i += length

			default:
				return Patch{}, ErrBadPatch
			}

		case ' ', '=':
			for s[i] != '=' {
				i++
				if i == len(s) {
					return Patch{}, ErrBadPatch
				}
			}
			i++
			for s[i] != ' ' {
				i++
				if i == len(s) {
					return Patch{}, ErrBadPatch
				}
			}
			i++
			patch.ValueJSON = make([]byte, len(s[i:]))
			copy(patch.ValueJSON, s[i:])
			return patch, nil

		default:
			return Patch{}, ErrBadPatch
		}
	}
	return Patch{}, errors.WithStack(ErrBadPatch)
}

func ParsePatchPath(s []byte) ([]byte, state.Keypath, *state.Range, error) {
	var keypath state.Keypath
	var rng *state.Range
	var i int
	for i = 0; i < len(s); {
		switch s[i] {
		case '.':
			key, err := parseDotKey(s[i:])
			if err != nil {
				return nil, nil, nil, err
			}
			keypath = keypath.Push(key)
			i += len(key) + 1

		case '[':
			switch s[i+1] {
			case '"', '\'':
				key, err := parseBracketKey(s[i:])
				if err != nil {
					return nil, nil, nil, err
				}
				keypath = keypath.Push(key)
				i += len(key) + 4

			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				var idx uint64
				var length int
				var err error
				rng, idx, length, err = parseRangeOrIndex(s[i:])
				if err != nil {
					return nil, nil, nil, err
				}
				if rng == nil {
					keypath = keypath.PushIndex(idx)
				}
				i += length
			}
		}
	}
	return s[i:], keypath, rng, nil
}

func parseDotKey(s []byte) ([]byte, error) {
	buf := []byte{}
	// start at index 1, skip first dot
	for i := 1; i < len(s); i++ {
		if s[i] == '.' || s[i] == '[' || s[i] == ' ' {
			if len(buf) == 0 {
				return nil, nil
			}
			return buf, nil
		} else {
			buf = append(buf, s[i])
		}
	}
	return buf, nil
}

func parseBracketKey(s []byte) ([]byte, error) {
	if len(s) < 5 {
		return nil, errors.WithStack(ErrBadPatch)
	} else if s[0] != '[' && s[1] != '"' {
		return nil, errors.WithStack(ErrBadPatch)
	}

	buf := []byte{}
	// start at index 2, skip ["
	for i := 2; i < len(s); i++ {
		if s[i] == '"' {
			return buf, nil
		} else {
			buf = append(buf, s[i])
		}
	}
	return nil, errors.WithStack(ErrBadPatch)
}

func parseRangeOrIndex(s []byte) (*state.Range, uint64, int, error) {
	var (
		isRange = false
		rng     = &state.Range{}
		buf     = make([]byte, 0, 8) // Approximation/heuristic
	)
	// Start at index 1, skip [
	for i := 1; i < len(s); i++ {
		if s[i] == ']' {
			if len(buf) == 0 {
				return nil, 0, 0, errors.WithStack(ErrBadPatch)
			}
			end, err := strconv.ParseInt(string(buf), 10, 64)
			if err != nil {
				return nil, 0, 0, errors.WithStack(ErrBadPatch)
			}

			if !isRange {
				if end < 0 {
					// Can't have negative indices (yet... @@TODO)
					return nil, 0, 0, errors.WithStack(ErrBadPatch)
				}
				return nil, uint64(end), i + 1, nil
			}

			if end == 0 && buf[0] == '-' {
				rng.Reverse = true
			} else if end < 0 {
				rng.End = uint64(-end)
				rng.Reverse = true
			} else {
				rng.End = uint64(end)
			}
			return rng, 0, i + 1, nil

		} else if s[i] == ':' {
			if isRange {
				// Disallow [x:y:z]
				return nil, 0, 0, errors.WithStack(ErrBadPatch)
			}
			isRange = true

			start, err := strconv.ParseInt(string(buf), 10, 64)
			if err != nil {
				return nil, 0, 0, errors.WithStack(ErrBadPatch)
			}
			if start == 0 && buf[0] == '-' {
				rng.Reverse = true
			} else if start < 0 {
				rng.Start = uint64(-start)
				rng.Reverse = true
			} else {
				rng.Start = uint64(start)
			}
			isRange = true
			buf = make([]byte, 0, 8)
		} else {
			buf = append(buf, s[i])
		}
	}
	return nil, 0, 0, errors.WithStack(ErrBadPatch)
}
