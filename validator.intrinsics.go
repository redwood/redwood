package redwood

import (
	"github.com/pkg/errors"
)

type intrinsicsValidator struct{}

func NewIntrinsicsValidator(params map[string]interface{}) (Validator, error) {
	return &intrinsicsValidator{}, nil
}

var ErrNoParentYet = errors.New("no parent yet")

func (v *intrinsicsValidator) Validate(state interface{}, txs, validTxs map[ID]*Tx, tx Tx) error {
	if len(tx.Parents) == 0 {
		return errors.New("tx must have parents")
	} else if len(validTxs) > 0 && len(tx.Parents) == 1 && tx.Parents[0] == GenesisTxID {
		return errors.New("already have a genesis tx")
	}

	for _, parentID := range tx.Parents {
		if _, exists := validTxs[parentID]; !exists && parentID.Pretty() != GenesisTxID.Pretty() {
			return errors.Wrapf(ErrNoParentYet, "txid: %v", parentID.Pretty())
		}
	}
	return nil
}
