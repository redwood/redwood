package redwood

// import (
// 	"github.com/pkg/errors"
// )

// type stackValidator struct {
// 	validators []Validator
// }

// func NewStackValidator(params map[string]interface{}) (Validator, error) {
// 	children, exists := M(params).GetSlice("children")
// 	if !exists {
// 		return nil, errors.New("stack validator needs an array 'children' param")
// 	}

// 	var validators []Validator
// 	for i := range children {
// 		config, is := children[i].(map[string]interface{})
// 		if !is {
// 			return nil, errors.New("stack validator found something that didn't look like a validator config")
// 		}

// 		validator, err := initValidatorFromConfig(config)
// 		if err != nil {
// 			return nil, err
// 		}

// 		validators = append(validators, validator)
// 	}

// 	return &stackValidator{validators: validators}, nil
// }

// func (v *stackValidator) ValidateFetch(state interface{}, keypath []string, requester Address) error {
// 	for i := range v.validators {
// 		err := v.validators[i].ValidateFetch(state, keypath, requester)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// func (v *stackValidator) ValidateTx(state interface{}, txs, validTxs map[ID]*Tx, tx Tx) error {
// 	for i := range v.validators {
// 		err := v.validators[i].ValidateTx(state, txs, validTxs, tx)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }
