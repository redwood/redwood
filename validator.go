package main

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type Validator interface {
	Validate(state interface{}, timeDAG map[ID]map[ID]bool, tx Tx) error
}

type validator struct {
	ID ID
}

func (v *validator) Validate(state interface{}, timeDAG map[ID]map[ID]bool, tx Tx) error {
	j, _ := json.MarshalIndent(state, "", "    ")
	log.Warnln("Validate", v.ID, string(j))

	if len(tx.Parents) == 0 {
		return errors.New("tx must have parents")
	} else if len(timeDAG) > 0 && len(tx.Parents) == 1 && tx.Parents[0] == GenesisTxID {
		return errors.New("already have a genesis tx")
	}

	// Check write permissions on each patch key
	stateMap, isMap := state.(map[string]interface{})
	if !isMap {
		return errors.New("no permissions configured")
	}
	maybePerms, exists := valueAtKeypath(stateMap, []string{"permissions", tx.From.String()})
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
