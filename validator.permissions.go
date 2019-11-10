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

func (v *permissionsValidator) canRead(perms map[string]interface{}, keypathStr string) (bool, error) {
	for pattern := range perms {
		matched, err := regexp.MatchString(pattern, keypathStr)
		if err != nil {
			return false, errors.Wrapf(Err403, "error executing regex")
		}

		if matched {
			if asBool, isBool := getBool(perms, []string{pattern, "read"}); isBool {
				return asBool, nil
			}
		}
	}
	return false, nil
}

func (v *permissionsValidator) PruneForbiddenPatches(state interface{}, patches []Patch, requester Address) ([]Patch, error) {
	asMap, isMap := state.(map[string]interface{})
	if !isMap {
		return nil, errors.Wrapf(Err403, "'state' is not a map")
	}
	perms, err := v.permsForUser(asMap, requester)
	if err != nil {
		return nil, err
	}

	var prunedPatches []Patch
	for _, patch := range patches {
		keypathStr := KeypathSeparator + strings.Join(patch.Keys, KeypathSeparator)
		// @@TODO: patch.Range?
		// @@TODO: patch.Val?
		canRead, err := v.canRead(perms, keypathStr)
		if err != nil {
			return nil, err
		}

		if canRead {
			prunedPatches = append(prunedPatches, patch)
		}
	}
	return prunedPatches, nil
}

func (v *permissionsValidator) PruneForbiddenState(state interface{}, requestedKeypath []string, requester Address) error {
	asMap, isMap := state.(map[string]interface{})
	if !isMap {
		return errors.Wrapf(Err403, "'state' is not a map")
	}
	perms, err := v.permsForUser(asMap, requester)
	if err != nil {
		return err
	}

	// Don't clobber perms while pruning forbidden paths
	perms = DeepCopyJSValue(perms).(map[string]interface{})

	// defaults, isMap := getMap(perms, []string{"defaults"})
	// if !isMap {
	// 	defaults := map[string]interface{}{
	// 		"read":  false,
	// 		"write": false,
	// 	}
	// }

	toWalk, exists := getValue(asMap, requestedKeypath)
	if !exists {
		return nil
	}

	requestedKeypathStr := KeypathSeparator + strings.Join(requestedKeypath, KeypathSeparator)

	switch toWalk.(type) {
	case map[string]interface{}, []interface{}:
		err := walkTree2(toWalk, func(keypath []string, parent interface{}, val interface{}) error {
			if len(keypath) == 0 {
				return nil
			}

			keypathStr := requestedKeypathStr + strings.Join(keypath, KeypathSeparator)
			canRead, err := v.canRead(perms, keypathStr)
			if err != nil {
				return err
			}
			if !canRead {
				if asMap, isMap := parent.(map[string]interface{}); isMap {
					delete(asMap, keypath[len(keypath)-1])
					// } else if asSlice, isSlice := parent.([]interface{}); isSlice {
					// @@TODO: ...?
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

	default:
		canRead, err := v.canRead(perms, requestedKeypathStr)
		if err != nil {
			return err
		}
		if !canRead {
			setValueAtKeypath(asMap, requestedKeypath, nil, false)
		}

	}

	return nil
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
