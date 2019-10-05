package redwood

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type PermissionsValidator struct{}

var Err403 = errors.New("nope")

func patchStrs(patches []Patch) []string {
	var s []string
	for i := range patches {
		s = append(s, patches[i].String())
	}
	return s
}

func (v *PermissionsValidator) Validate(state interface{}, timeDAG map[ID]map[ID]bool, tx Tx) error {
	maybePerms, exists := valueAtKeypath(state, []string{"permissions", tx.From.String()})
	if !exists {
		return errors.WithStack(Err403)
	}
	perms, isMap := maybePerms.(map[string]interface{})
	if !isMap {
		return errors.WithStack(Err403)
	}

	for _, patch := range tx.Patches {
		var valid bool

		keypath := strings.Join(patch.Keys, "/")
		for pattern := range perms {
			matched, err := regexp.MatchString(pattern, keypath)
			if err != nil {
				return errors.WithStack(Err403)
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
			return errors.WithStack(Err403)
		}
	}

	return nil
}
