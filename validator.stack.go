package redwood

type stackValidator struct {
	validators []Validator
}

func NewStackValidator(validators []Validator) Validator {
	return &stackValidator{validators: validators}
}

func (v *stackValidator) Validate(state interface{}, timeDAG map[ID]map[ID]bool, tx Tx) error {
	for i := range v.validators {
		err := v.validators[i].Validate(state, timeDAG, tx)
		if err != nil {
			return err
		}
	}
	return nil
}
