package tree

import (
	"github.com/pkg/errors"

	"redwood.dev/state"
)

type keypathIndexer struct {
	keypathName state.Keypath
}

func NewKeypathIndexer(config state.Node) (Indexer, error) {
	keypathStr, exists, err := config.StringValue(state.Keypath("keypath"))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.New("keypath indexer needs a 'keypath' field in its config")
	}
	return &keypathIndexer{keypathName: state.Keypath(keypathStr)}, nil
}

func (i *keypathIndexer) IndexNode(relKeypath state.Keypath, node state.Node) (state.Keypath, state.Node, error) {
	val, exists, err := node.Value(i.keypathName, nil)
	if err != nil {
		return nil, nil, err
	} else if !exists {
		return nil, nil, nil
	}

	valStr, is := val.(string)
	if !is {
		return nil, nil, nil
	}

	return state.Keypath(valStr), node, nil
}
