package redwood

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type permissionsValidator struct {
	permissions map[string]interface{}
}

func NewPermissionsValidator(params map[string]interface{}) (Validator, error) {
	permissions, exists := getMap(params, []string{"permissions"})
	if !exists {
		return nil, errors.New("permissions validator needs a 'permissions' param")
	}

	return &permissionsValidator{permissions: permissions}, nil
}

var Err403 = errors.New("403: nope")

func (v *permissionsValidator) ValidateTx(state interface{}, txs, validTxs map[ID]*Tx, tx Tx) error {
	perms, exists := v.permissions[tx.From.Hex()]
	if !exists {
		perms, exists = v.permissions["*"]
		if !exists {
			return errors.WithStack(errors.Wrapf(Err403, "permissions key for user '%v' does not exist", tx.From.Hex()))
		}
	}
	permsMap, isMap := perms.(map[string]interface{})
	if !isMap {
		return errors.WithStack(errors.Wrapf(Err403, "permissions key for user '%v' does not contain a map", tx.From.Hex()))
	}

	for _, patch := range tx.Patches {
		var valid bool

		keypath := KeypathSeparator + strings.Join(patch.Keys, KeypathSeparator)
		for pattern := range permsMap {
			matched, err := regexp.MatchString(pattern, keypath)
			if err != nil {
				return errors.Wrapf(Err403, "error executing regex")
			}

			if matched {
				canWrite, _ := getValue(permsMap, []string{pattern, "write"})
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
