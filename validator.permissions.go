package redwood

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type permissionsValidator struct {
	permissions map[string]interface{}
}

func NewPermissionsValidator(r RefResolver, config *NelSON) (Validator, error) {
	asMap, isMap := config.Value().(map[string]interface{})
	if !isMap {
		return nil, errors.New("permissions validator needs a map of permissions as its config")
	}
	return &permissionsValidator{permissions: asMap}, nil
}

var senderRegexp = regexp.MustCompile("\\${sender}")

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
			expandedPattern := string(senderRegexp.ReplaceAll([]byte(pattern), []byte(tx.From.Hex())))
			matched, err := regexp.MatchString(expandedPattern, keypath)
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
