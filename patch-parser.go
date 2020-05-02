package redwood

import (
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/tree"
)

var ErrBadPatch = errors.New("bad patch string")

func ParsePatch(s []byte) (Patch, error) {
	patch := Patch{}

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

func ParsePatchPath(s []byte) ([]byte, tree.Keypath, *tree.Range, error) {
	var keypath tree.Keypath
	var rng *tree.Range
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
				var length int
				var err error
				rng, length, err = parseRange(s[i:])
				if err != nil {
					return nil, nil, nil, err
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

func parseRange(s []byte) (*tree.Range, int, error) {
	rng := &tree.Range{}
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
			rng[1] = end
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
			rng[0] = start
			haveStart = true
		} else {
			buf = append(buf, s[i])
		}
	}
	return nil, 0, errors.WithStack(ErrBadPatch)
}
