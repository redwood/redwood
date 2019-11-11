package redwood

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type permissionsValidator struct {
	name string
}

func NewPermissionsValidator(params map[string]interface{}) (Validator, error) {
	return &permissionsValidator{}, nil
}

var Err403 = errors.New("403: nope")

func (v *permissionsValidator) permsForUser(state map[string]interface{}, user Address) (map[string]interface{}, error) {
	perms, exists := getMap(state, []string{"permissions", user.Hex()})
	if !exists {
		perms, exists = getMap(state, []string{"permissions", "*"})
		if !exists {
			return nil, errors.WithStack(errors.Wrapf(Err403, "permissions key for user '%v' does not exist", user.Hex()))
		}
	}
	return perms, nil
}

func (v *permissionsValidator) ValidateTx(state interface{}, txs, validTxs map[ID]*Tx, tx Tx) error {
	asMap, isMap := state.(map[string]interface{})
	if !isMap {
		return errors.Wrapf(Err403, "'state' is not a map")
	}
	perms, err := v.permsForUser(asMap, tx.From)
	if err != nil {
		return err
	}

	for _, patch := range tx.Patches {
		var valid bool

		keypath := KeypathSeparator + strings.Join(patch.Keys, KeypathSeparator)
		for pattern := range perms {
			matched, err := regexp.MatchString(pattern, keypath)
			if err != nil {
				return errors.Wrapf(Err403, "error executing regex")
			}

			if matched {
				canWrite, _ := getValue(perms, []string{pattern, "write"})
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
