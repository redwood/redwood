package nelson

import (
	"redwood.dev/state"
)

func (r Resolver) HelperDrillDownUntilFrame(node state.Node, keypath state.Keypath) (frameNode, nonFrameNode state.Node, remaining state.Keypath, _ error) {
	return r.drillDownUntilFrame(node, keypath)
}

func (r Resolver) HelperCollapseBasicFrame(node state.Node, keypath state.Keypath) (Frame, state.Keypath, error) {
	return r.collapseBasicFrame(node, keypath)
}
