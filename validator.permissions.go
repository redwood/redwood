package redwood

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/nelson"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type permissionsValidator struct {
	permissions map[string]interface{}
}

func NewPermissionsValidator(config tree.Node) (Validator, error) {
	cfg, exists, err := nelson.GetValueRecursive(config, nil, nil)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.New("permissions validator needs a map of permissions as its config")
	}

	asMap, isMap := cfg.(map[string]interface{})
	if !isMap {
		return nil, errors.New("permissions validator needs a map of permissions as its config")
	}
	lowercasePerms := make(map[string]interface{}, len(asMap))
	for address, perms := range asMap {
		lowercasePerms[strings.ToLower(address)] = perms
	}
	return &permissionsValidator{permissions: lowercasePerms}, nil
}

var senderRegexp = regexp.MustCompile("\\${sender}")

func (v *permissionsValidator) ValidateTx(state tree.Node, tx *Tx) error {
	perms, exists := v.permissions[strings.ToLower(tx.From.Hex())]
	if !exists {
		perms, exists = v.permissions["*"]
		if !exists {
			return errors.WithStack(errors.Wrapf(types.Err403, "permissions key for user '%v' does not exist", tx.From.Hex()))
		}
	}
	permsMap, isMap := perms.(map[string]interface{})
	if !isMap {
		return errors.WithStack(errors.Wrapf(types.Err403, "permissions key for user '%v' does not contain a map", tx.From.Hex()))
	}

	for _, patch := range tx.Patches {
		var valid bool

		// @@TODO: hacky
		keypath := KeypathSeparator + string(bytes.ReplaceAll(patch.Keypath, tree.KeypathSeparator, []byte(KeypathSeparator)))
		for pattern := range permsMap {
			expandedPattern := string(senderRegexp.ReplaceAll([]byte(pattern), []byte(tx.From.Hex())))
			matched, err := regexp.MatchString(expandedPattern, keypath)
			if err != nil {
				return errors.Wrapf(types.Err403, "error executing regex")
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
			return errors.Wrapf(types.Err403, "could not find a matching rule (user: %v, patch: %v)", tx.From.String(), patch.String())
		}
	}

	return nil
}
