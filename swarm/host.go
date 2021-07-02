package swarm

import (
	"redwood.dev/config"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/tree"
)

type Host interface {
	log.Logger
	Start() error
	Close()
	Transport(name string) Transport
	Protocol(name string) Protocol
	PeerStore() PeerStore
	KeyStore() identity.KeyStore
	Controllers() tree.ControllerHub
}

type host struct {
	log.Logger
	chStop chan struct{}
	chDone chan struct{}

	config *config.Config

	transports    map[string]Transport
	protocols     map[string]Protocol
	peerStore     PeerStore
	keyStore      identity.KeyStore
	controllerHub tree.ControllerHub
}

func NewHost(
	transports []Transport,
	protocols []Protocol,
	peerStore PeerStore,
	keyStore identity.KeyStore,
	controllerHub tree.ControllerHub,
	config *config.Config,
) (Host, error) {
	transportsMap := make(map[string]Transport)
	for _, tpt := range transports {
		transportsMap[tpt.Name()] = tpt
	}
	protocolsMap := make(map[string]Protocol)
	for _, proto := range protocols {
		protocolsMap[proto.Name()] = proto
	}
	h := &host{
		Logger:        log.NewLogger("host"),
		chStop:        make(chan struct{}),
		chDone:        make(chan struct{}),
		transports:    transportsMap,
		protocols:     protocolsMap,
		peerStore:     peerStore,
		keyStore:      keyStore,
		controllerHub: controllerHub,
		config:        config,
	}
	return h, nil
}

func (h *host) Start() error {
	h.SetLogLabel("host")

	// Set up the transports
	for _, transport := range h.transports {
		err := transport.Start()
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *host) Close() {
	close(h.chStop)

	for _, tpt := range h.transports {
		tpt.Close()
	}

	for _, proto := range h.protocols {
		proto.Close()
	}
}

func (h *host) Transport(name string) Transport {
	return h.transports[name]
}

func (h *host) Protocol(name string) Protocol {
	return h.protocols[name]
}

func (h *host) PeerStore() PeerStore {
	return h.peerStore
}

func (h *host) KeyStore() identity.KeyStore {
	return h.keyStore
}

func (h *host) Controllers() tree.ControllerHub {
	return h.controllerHub
}
