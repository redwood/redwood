package redwood

type Validator interface {
	Validate(state interface{}, timeDAG map[ID]map[ID]bool, tx Tx) error
}
