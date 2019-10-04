package redwood

import (
	"github.com/pkg/errors"
)

type IntrinsicsValidator struct{}

func (v *IntrinsicsValidator) Validate(state interface{}, timeDAG map[ID]map[ID]bool, tx Tx) error {
	if len(tx.Parents) == 0 {
		return errors.New("tx must have parents")
	} else if len(timeDAG) > 0 && len(tx.Parents) == 1 && tx.Parents[0] == GenesisTxID {
		return errors.New("already have a genesis tx")
	}
	return nil
}
