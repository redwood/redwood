package redwood

import (
	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/tree"
)

type keypathIndexer struct {
	keypathName tree.Keypath
}

func NewKeypathIndexer(config tree.Node) (Indexer, error) {
	keypathStr, exists, err := config.StringValue(tree.Keypath("keypath"))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.New("keypath indexer needs a 'keypath' field in its config")
	}
	return &keypathIndexer{keypathName: tree.Keypath(keypathStr)}, nil
}

func (i *keypathIndexer) IndexNode(relKeypath tree.Keypath, node tree.Node) (tree.Keypath, tree.Node, error) {
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

	return tree.Keypath(valStr), node, nil
}
