package redwood

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type PermissionsValidator struct{}

func (v *PermissionsValidator) Validate(state interface{}, timeDAG map[ID]map[ID]bool, tx Tx) error {
	// Check write permissions on each patch key
	maybePerms, exists := valueAtKeypath(state, []string{"permissions", tx.From.String()})
	if !exists {
		return errors.New("no permissions configured")
	}
	perms, isMap := maybePerms.(map[string]interface{})
	if !isMap {
		return errors.New("no permissions configured")
	}

	for _, patch := range tx.Patches {
		var valid bool

		keypath := strings.Join(patch.Keys, "/")
		for pattern := range perms {
			matched, err := regexp.MatchString(pattern, keypath)
			if err != nil {
				return errors.WithStack(err)
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
			return errors.New("nope")
		}
	}

	return nil
}
