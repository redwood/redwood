package redwood

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type permissionsValidator struct{}

func NewPermissionsValidator(params map[string]interface{}) (Validator, error) {
	return &permissionsValidator{}, nil
}

var Err403 = errors.New("403: nope")

func patchStrs(patches []Patch) []string {
	var s []string
	for i := range patches {
		s = append(s, patches[i].String())
	}
	return s
}

func (v *permissionsValidator) Validate(state interface{}, txs, validTxs map[ID]*Tx, tx Tx) error {
	asMap, isMap := state.(map[string]interface{})
	if !isMap {
		return errors.Wrapf(Err403, "'state' is not a map")
	}
	maybePerms, exists := valueAtKeypath(asMap, []string{"permissions", tx.From.Hex()})
	if !exists {
		maybePerms, exists = valueAtKeypath(asMap, []string{"permissions", "*"})
		if !exists {
			return errors.Wrapf(Err403, "permissions key for user '%v' does not exist", tx.From.String())
		}
	}
	perms, isMap := maybePerms.(map[string]interface{})
	if !isMap {
		return errors.Wrapf(Err403, "permissions key for user '%v' is not a map", tx.From.String())
	}

	for _, patch := range tx.Patches {
		var valid bool

		keypath := strings.Join(patch.Keys, "/")
		for pattern := range perms {
			matched, err := regexp.MatchString(pattern, keypath)
			if err != nil {
				return errors.Wrapf(Err403, "error executing regex")
			}

			if matched {
				canWrite, _ := valueAtKeypath(perms, []string{pattern, "write"})
				if canWrite == true {
					valid = true
					break
				}
			}
		}
		if !valid {
			return errors.Wrapf(Err403, "could not find a matching rule (user: %v, patch: %v)", tx.From.String(), patch.String())
		}
	}

	return nil
}
