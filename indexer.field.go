package redwood

import (
	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/nelson"
	"github.com/brynbellomy/redwood/tree"
)

type keypathIndexer struct {
	keypathName tree.Keypath
}

func NewKeypathIndexer(config tree.Node) (Indexer, error) {
	cfg, exists, err := nelson.GetValueRecursive(config, nil, nil)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.New("keypath indexer is missing its config")
	}

	asMap, isMap := cfg.(map[string]interface{})
	if !isMap {
		return nil, errors.New("keypath indexer needs a map as its config")
	}

	keypathVal := asMap["keypath"]

	keypathStr, is := keypathVal.(string)
	if !is {
		return nil, errors.New("keypath indexer is missing its config")
	}
	return &keypathIndexer{keypathName: tree.Keypath(keypathStr)}, nil
}

func (i *keypathIndexer) IndexKeyForNode(node tree.Node) (tree.Keypath, error) {
	val, exists, err := node.Value(i.keypathName, nil)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}

	valStr, is := val.(string)
	if !is {
		return nil, nil
	}

	return tree.Keypath(valStr), nil
}
