package redwood

import (
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
)

var ErrBadPatch = errors.New("bad patch string")

func ParsePatch(s string) (Patch, error) {
	patch := Patch{}

	for i := 0; i < len(s); {
		switch s[i] {
		case '.':
			key, err := parseDotKey(s[i:])
			if err != nil {
				return Patch{}, err
			}
			patch.Keys = append(patch.Keys, key)
			i += len(key) + 1

		case '[':
			switch s[i+1] {
			case '"', '\'':
				key, err := parseBracketKey(s[i:])
				if err != nil {
					return Patch{}, err
				}
				patch.Keys = append(patch.Keys, key)
				i += len(key) + 4

			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				rng, length, err := parseRange(s[i:])
				if err != nil {
					return Patch{}, err
				}
				patch.Range = rng
				i += length
			}

		case ' ', '=':
			for s[i] != '=' {
				i++
			}
			i++
			err := json.Unmarshal([]byte(s[i:]), &patch.Val)
			if err != nil {
				return Patch{}, errors.Wrapf(ErrBadPatch, err.Error())
			}
			return patch, nil
		}
	}
	return Patch{}, errors.WithStack(ErrBadPatch)
}

func parseDotKey(s string) (string, error) {
	buf := []byte{}
	// start at index 1, skip first dot
	for i := 1; i < len(s); i++ {
		if s[i] == '.' || s[i] == '[' || s[i] == ' ' {
			return string(buf), nil
		} else {
			buf = append(buf, s[i])
		}
	}
	return "", errors.WithStack(ErrBadPatch)
}

func parseBracketKey(s string) (string, error) {
	if len(s) < 5 {
		return "", errors.WithStack(ErrBadPatch)
	} else if s[0] != '[' && s[1] != '"' {
		return "", errors.WithStack(ErrBadPatch)
	}

	buf := []byte{}
	// start at index 2, skip ["
	for i := 2; i < len(s); i++ {
		if s[i] == '"' {
			return string(buf), nil
		} else {
			buf = append(buf, s[i])
		}
	}
	return "", errors.WithStack(ErrBadPatch)
}

func parseRange(s string) (*Range, int, error) {
	rng := &Range{}
	haveStart := false
	buf := []byte{}
	// start at index 1, skip [
	for i := 1; i < len(s); i++ {
		if s[i] == ']' {
			if !haveStart {
				return nil, 0, errors.WithStack(ErrBadPatch)
			}
			end, err := strconv.ParseInt(string(buf), 10, 64)
			if err != nil {
				return nil, 0, errors.WithStack(ErrBadPatch)
			}
			rng.End = end
			return rng, i + 1, nil

		} else if s[i] == ':' {
			if haveStart {
				return nil, 0, errors.WithStack(ErrBadPatch)
			}

			start, err := strconv.ParseInt(string(buf), 10, 64)
			if err != nil {
				return nil, 0, errors.WithStack(ErrBadPatch)
			}
			buf = []byte{}
			rng.Start = start
			haveStart = true
		} else {
			buf = append(buf, s[i])
		}
	}
	return nil, 0, errors.WithStack(ErrBadPatch)
}
