package pb

import (
	"bytes"

	"redwood.dev/errors"
	"redwood.dev/state"
)

var ErrBadPatch = errors.New("bad patch string")
var equalsSign byte = '='

func ParsePatch(s []byte) (Patch, error) {
	idx := bytes.IndexByte(s, equalsSign)
	if idx < 0 {
		return Patch{}, errors.Wrapf(ErrBadPatch, "no '=' sign")
	}

	keypath, rng, err := state.ParseKeypathAndRange(s[:idx], KeypathSeparator[0])
	if err != nil {
		return Patch{}, errors.Wrapf(err, "%v", ErrBadPatch)
	}

	valueJSON := bytes.TrimSpace(s[idx+1:])
	if len(valueJSON) == 0 {
		return Patch{}, errors.Wrapf(ErrBadPatch, "no value")
	}

	return Patch{
		Keypath:   keypath,
		Range:     rng,
		ValueJSON: valueJSON,
	}, nil
}
