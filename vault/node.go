package vault

import (
	"time"

	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/types"
	. "redwood.dev/utils/generics"
)

type Node struct {
	process.Process
	log.Logger
	server *Server
	store  Store
}

func NewNode(server *Server, store Store) *Node {
	return &Node{
		Process: *process.New("vault node"),
		Logger:  log.NewLogger("vault node"),
		server:  server,
		store:   store,
	}
}

func (n *Node) Start() error {
	err := n.Process.Start()
	if err != nil {
		return err
	}
	return n.Process.SpawnChild(nil, n.server)
}

func (n *Node) Collections() ([]string, error) {
	return n.store.Collections()
}

func (n *Node) Items(collectionID string, oldestMtime time.Time, start, end uint64) ([]Item, error) {
	return n.store.Items(collectionID, oldestMtime, start, end)
}

func (n *Node) DeleteAll() error {
	return n.store.DeleteAll()
}

func (n *Node) UserCapabilities(addr types.Address) Set[Capability] {
	return n.server.accessControl.UserCapabilities(addr)
}

func (n *Node) SetUserCapabilities(addr types.Address, capabilities []Capability) {
	n.server.accessControl.SetCapabilities(addr, capabilities)
}
