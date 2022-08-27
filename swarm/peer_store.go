package swarm

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"math"
	"sync"
	"time"

	"redwood.dev/crypto"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/types"
	"redwood.dev/utils"
	. "redwood.dev/utils/generics"
)

//go:generate mockery --name PeerStore --output ./mocks/ --case=underscore
type PeerStore interface {
	process.Interface

	AddDialInfo(dialInfo PeerDialInfo, deviceUniqueID string) (PeerDevice, PeerEndpoint)
	AddVerifiedCredentials(dialInfo PeerDialInfo, deviceUniqueID string, address types.Address, sigpubkey *crypto.SigningPublicKey, encpubkey *crypto.AsymEncPubkey) (PeerDevice, PeerEndpoint)
	RemovePeers(deviceUniqueIDs []string) error
	UnverifiedPeers() []PeerDevice
	VerifiedPeers() []PeerDevice
	Peers() []PeerDevice
	AllDialInfos() Set[PeerDialInfo]
	PeerWithDeviceUniqueID(deviceUniqueID string) (PeerDevice, bool)
	PeerEndpoint(dialInfo PeerDialInfo) PeerEndpoint
	PeersWithAddress(address types.Address) []PeerDevice
	PeersFromTransport(transportName string) []PeerEndpoint
	PeersServingStateURI(stateURI string) []PeerDevice
	IsKnownPeer(dialInfo PeerDialInfo) bool
	OnNewUnverifiedPeer(fn func(dialInfo PeerDialInfo))
	OnNewVerifiedPeer(fn func(peer PeerDevice))

	DebugPrint()
}

type peerStore struct {
	process.Process
	log.Logger

	state                   *state.DBTree
	muPeers                 sync.RWMutex
	peerEndpoints           map[PeerDialInfo]*peerEndpoint
	deviceIDsWithAddress    map[types.Address]Set[string]
	peersWithDeviceUniqueID map[string]*peerDevice
	unverifiedDevices       Set[*peerDevice]

	newUnverifiedPeerListeners   []func(dialInfo PeerDialInfo)
	newUnverifiedPeerListenersMu sync.RWMutex
	newVerifiedPeerListeners     []func(peer PeerDevice)
	newVerifiedPeerListenersMu   sync.RWMutex
}

func NewPeerStore(db *state.DBTree) *peerStore {
	s := &peerStore{
		Process:                 *process.New("peerstore"),
		Logger:                  log.NewLogger("peerstore"),
		state:                   db,
		peerEndpoints:           make(map[PeerDialInfo]*peerEndpoint),
		deviceIDsWithAddress:    make(map[types.Address]Set[string]),
		peersWithDeviceUniqueID: make(map[string]*peerDevice),
		unverifiedDevices:       NewSet[*peerDevice](nil),
	}
	s.Infof("opening peer store")

	node := s.state.State(false)
	defer node.Close()

	keypath := state.Keypath("peers")

	var peerDevices map[string]*peerDevice
	err := node.NodeAt(keypath, nil).Scan(&peerDevices)
	if err != nil {
		s.Warnf("could not fetch stored peer details from DB: %v", err)
	} else {
		for _, device := range peerDevices {
			if device == nil {
				continue
			}
			device.peerStore = s
			for _, e := range device.Endpts {
				if e == nil {
					continue
				}
				e.peerDevice = device
				s.peerEndpoints[e.Dialinfo] = e

				if len(device.DeviceUniqID) > 0 {
					s.peersWithDeviceUniqueID[device.DeviceUniqID] = device
				}

				if len(device.Addrs) > 0 {
					for addr := range device.Addrs {
						if _, exists := s.deviceIDsWithAddress[addr]; !exists {
							s.deviceIDsWithAddress[addr] = NewSet[string](nil)
						}
						s.deviceIDsWithAddress[addr].Add(device.DeviceUniqID)
					}

				} else {
					s.unverifiedDevices.Add(device)
				}
			}
		}
	}
	return s
}

func (s *peerStore) Start() error {
	err := s.Process.Start()
	if err != nil {
		return err
	}

	s.Process.Go(nil, "periodically persist to disk", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return

			case <-time.After(15 * time.Second):
				err := func() error {
					s.muPeers.RLock()
					defer s.muPeers.RUnlock()

					node := s.state.State(true)
					defer node.Close()

					err := node.Set(state.Keypath("peers"), nil, s.peersWithDeviceUniqueID)
					if err != nil {
						return err
					}
					return node.Save()
				}()
				if err != nil {
					s.Errorf("while persisting peer store to disk: %+v", err)
				}
			}
		}
	})

	return nil
}

func (s *peerStore) Peers() []PeerDevice {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()
	return Map[[]*peerDevice, []PeerDevice, *peerDevice, PeerDevice](Values(s.peersWithDeviceUniqueID), func(p *peerDevice) PeerDevice { return p })
}

func (s *peerStore) OnNewUnverifiedPeer(fn func(dialInfo PeerDialInfo)) {
	s.newUnverifiedPeerListenersMu.Lock()
	defer s.newUnverifiedPeerListenersMu.Unlock()
	s.newUnverifiedPeerListeners = append(s.newUnverifiedPeerListeners, fn)
}

func (s *peerStore) notifyNewUnverifiedPeerListeners(dialInfo PeerDialInfo) {
	s.newUnverifiedPeerListenersMu.RLock()
	defer s.newUnverifiedPeerListenersMu.RUnlock()
	for _, listener := range s.newUnverifiedPeerListeners {
		listener(dialInfo)
	}
}

func (s *peerStore) OnNewVerifiedPeer(fn func(peer PeerDevice)) {
	s.newVerifiedPeerListenersMu.Lock()
	defer s.newVerifiedPeerListenersMu.Unlock()
	s.newVerifiedPeerListeners = append(s.newVerifiedPeerListeners, fn)
}

func (s *peerStore) notifyNewVerifiedPeerListeners(peer PeerDevice) {
	s.newVerifiedPeerListenersMu.RLock()
	defer s.newVerifiedPeerListenersMu.RUnlock()
	for _, listener := range s.newVerifiedPeerListeners {
		listener(peer)
	}
}

func (s *peerStore) AddDialInfo(dialInfo PeerDialInfo, deviceUniqueID string) (PeerDevice, PeerEndpoint) {
	if dialInfo.DialAddr == "" {
		device := newPeerDevice(s, deviceUniqueID)
		endpoint := device.ensureEndpoint(dialInfo)
		return device, endpoint
	}

	var device *peerDevice
	var endpoint *peerEndpoint
	var knownDevice bool
	func() {
		s.muPeers.Lock()
		defer s.muPeers.Unlock()

		device, endpoint, knownDevice = s.ensurePeerDevice(dialInfo, deviceUniqueID)
		if len(device.Addrs) == 0 {
			s.unverifiedDevices.Add(device)
		}
	}()
	if !knownDevice {
		s.notifyNewUnverifiedPeerListeners(dialInfo)
	}
	return device, endpoint
}

func (s *peerStore) findPeerDevice(dialInfo PeerDialInfo, deviceUniqueID string) *peerDevice {
	if deviceUniqueID != "" {
		device, exists := s.peersWithDeviceUniqueID[deviceUniqueID]
		if exists {
			return device
		}
	}
	endpoint, exists := s.peerEndpoints[dialInfo]
	if !exists {
		return nil
	}
	return endpoint.peerDevice
}

func (s *peerStore) ensurePeerDevice(dialInfo PeerDialInfo, deviceUniqueID string) (device *peerDevice, endpoint *peerEndpoint, knownDevice bool) {
	device = s.findPeerDevice(dialInfo, deviceUniqueID)
	knownDevice = device != nil

	if !knownDevice {
		device = newPeerDevice(s, deviceUniqueID)
		s.unverifiedDevices.Add(device)
	}

	if deviceUniqueID != "" {
		s.peersWithDeviceUniqueID[deviceUniqueID] = device

		// Handle the case where a peer's deviceUniqueID has changed (cleared browser cache, wiped keystore, etc.)
		if device.DeviceUniqID != deviceUniqueID && device.DeviceUniqID != "" {
			for addr := range device.Addrs {
				if _, exists := s.deviceIDsWithAddress[addr]; !exists {
					s.deviceIDsWithAddress[addr] = NewSet[string](nil)
				}
				s.deviceIDsWithAddress[addr].Add(deviceUniqueID)
			}
			delete(s.peersWithDeviceUniqueID, device.DeviceUniqID)
		}
		device.DeviceUniqID = deviceUniqueID
	}
	endpoint = device.ensureEndpoint(dialInfo)
	s.peerEndpoints[dialInfo] = endpoint
	return
}

func (s *peerStore) AddVerifiedCredentials(
	dialInfo PeerDialInfo,
	deviceUniqueID string,
	address types.Address,
	sigpubkey *crypto.SigningPublicKey,
	encpubkey *crypto.AsymEncPubkey,
) (PeerDevice, PeerEndpoint) {
	if dialInfo.DialAddr == "" {
		device := newPeerDevice(s, "")
		endpoint := device.ensureEndpoint(dialInfo)

		device.Addrs.Add(address)
		device.Sigpubkeys[address] = sigpubkey
		device.Encpubkeys[address] = encpubkey

		return device, endpoint
	}

	if address.IsZero() {
		panic("cannot add verified peer without credentials")
	}

	var (
		device                *peerDevice
		endpoint              *peerEndpoint
		deviceAlreadyVerified bool
	)
	func() {
		s.muPeers.Lock()
		defer s.muPeers.Unlock()

		_, exists := s.deviceIDsWithAddress[address]
		if exists {
			deviceAlreadyVerified = s.deviceIDsWithAddress[address].Contains(deviceUniqueID)
		}

		device, endpoint, _ = s.ensurePeerDevice(dialInfo, deviceUniqueID)

		device.Addrs.Add(address)
		device.Sigpubkeys[address] = sigpubkey
		device.Encpubkeys[address] = encpubkey

		if _, exists := s.deviceIDsWithAddress[address]; !exists {
			s.deviceIDsWithAddress[address] = NewSet[string](nil)
		}
		s.deviceIDsWithAddress[address].Add(deviceUniqueID)

		if dialInfo.DialAddr != "" {
			s.peerEndpoints[dialInfo] = device.Endpts[dialInfo]
		}

		s.unverifiedDevices.Remove(device)
	}()

	if !deviceAlreadyVerified {
		s.notifyNewVerifiedPeerListeners(device)
	}
	return device, endpoint
}

func (s *peerStore) RemovePeers(deviceUniqueIDs []string) error {
	s.muPeers.Lock()
	defer s.muPeers.Unlock()

	for _, deviceUniqueID := range deviceUniqueIDs {
		device, exists := s.peersWithDeviceUniqueID[deviceUniqueID]
		if !exists {
			continue
		}
		for dialInfo := range device.Endpts {
			delete(s.peerEndpoints, dialInfo)
			s.unverifiedDevices.Remove(device)
		}
		for addr := range device.Addrs {
			device.Addrs.Remove(addr)
		}
		delete(s.peersWithDeviceUniqueID, device.DeviceUniqID)
	}
	return s.deletePeers(deviceUniqueIDs)
}

func (s *peerStore) PeerEndpoint(dialInfo PeerDialInfo) PeerEndpoint {
	if dialInfo.DialAddr == "" {
		return nil
	}
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()
	e, exists := s.peerEndpoints[dialInfo]
	if !exists {
		return nil
	}
	return e
}

func (s *peerStore) UnverifiedPeers() []PeerDevice {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()
	return Map[[]*peerDevice, []PeerDevice, *peerDevice, PeerDevice](s.unverifiedDevices.Slice(), func(d *peerDevice) PeerDevice { return d })
}

func (s *peerStore) VerifiedPeers() []PeerDevice {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var pds []PeerDevice
	for _, peers := range s.deviceIDsWithAddress {
		for deviceUniqueID := range peers {
			pd, exists := s.peersWithDeviceUniqueID[deviceUniqueID]
			if !exists {
				continue
			}
			pds = append(pds, pd)
		}
	}
	return pds
}

func (s *peerStore) AllDialInfos() Set[PeerDialInfo] {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	set := NewSet[PeerDialInfo](nil)
	set.AddAll(Keys(s.peerEndpoints)...)
	set.AddAll(FlatMap(s.unverifiedDevices.Slice(), func(d *peerDevice) []PeerDialInfo { return []PeerDialInfo(Keys(d.Endpts)) })...)
	return set
}

func (s *peerStore) PeerWithDeviceUniqueID(deviceUniqueID string) (PeerDevice, bool) {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()
	p, exists := s.peersWithDeviceUniqueID[deviceUniqueID]
	return p, exists
}

func (s *peerStore) PeersWithAddress(address types.Address) []PeerDevice {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []PeerDevice
	for deviceUniqueID := range s.deviceIDsWithAddress[address] {
		pd, exists := s.peersWithDeviceUniqueID[deviceUniqueID]
		if !exists {
			continue
		}
		peers = append(peers, pd)
	}
	return peers
}

func (s *peerStore) PeersFromTransport(transportName string) []PeerEndpoint {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peerEndpoints []PeerEndpoint
	for dialInfo, peerEndpoint := range s.peerEndpoints {
		if dialInfo.TransportName == transportName {
			peerEndpoints = append(peerEndpoints, peerEndpoint)
		}
	}
	return peerEndpoints
}

func (s *peerStore) PeersServingStateURI(stateURI string) []PeerDevice {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peerInfos []PeerDevice
	for _, p := range s.peersWithDeviceUniqueID {
		if !p.Stateuris.Contains(stateURI) {
			continue
		}
		peerInfos = append(peerInfos, p)
	}
	return peerInfos
}

func (s *peerStore) IsKnownPeer(dialInfo PeerDialInfo) bool {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()
	_, exists := s.peerEndpoints[dialInfo]
	return exists
}

func (s *peerStore) dialInfoHash(dialInfo PeerDialInfo) string {
	return types.HashBytes([]byte(dialInfo.TransportName + ":" + dialInfo.DialAddr)).Hex()
}

func (s *peerStore) deletePeers(deviceUniqueIDs []string) error {
	node := s.state.State(true)
	defer node.Close()

	for _, duID := range deviceUniqueIDs {
		peerKeypath := state.Keypath("peers").Pushs(duID)
		err := node.Delete(peerKeypath, nil)
		if err != nil {
			return err
		}
	}
	return node.Save()
}

func (s *peerStore) DebugPrint() {
	node := s.state.State(false)
	defer node.Close()
	node.NodeAt(state.Keypath("peers"), nil).DebugPrint(func(msg string, args ...interface{}) { fmt.Printf(msg, args...) }, true, 0)
	// s.Debugf("%v", utils.PrettyJSON(node.NodeAt(state.Keypath("peers"), nil)))
}

type PeerDevice interface {
	DeviceUniqueID() string
	SetDeviceUniqueID(id string)
	Addresses() Set[types.Address]
	PublicKeys(addr types.Address) (*crypto.SigningPublicKey, *crypto.AsymEncPubkey)
	StateURIs() Set[string]
	AddStateURI(stateURI string)
	RemoveStateURI(stateURI string)
	LastContact() time.Time
	LastFailure() time.Time
	Failures() uint64
	Ready() bool
	RemainingBackoff() time.Duration

	Endpoints() map[PeerDialInfo]PeerEndpoint
	Endpoint(dialInfo PeerDialInfo) (PeerEndpoint, bool)
}

type peerDevice struct {
	peerStore    *peerStore                                 `tree:"-"`
	DeviceUniqID string                                     `tree:"deviceUniqueID"`
	Addrs        Set[types.Address]                         `tree:"addresses"`
	Sigpubkeys   map[types.Address]*crypto.SigningPublicKey `tree:"sigpubkeys"`
	Encpubkeys   map[types.Address]*crypto.AsymEncPubkey    `tree:"encpubkeys"`
	Stateuris    Set[string]                                `tree:"stateURIs"`
	Endpts       map[PeerDialInfo]*peerEndpoint             `tree:"peerEndpoints"`
}

func newPeerDevice(peerStore *peerStore, deviceUniqueID string) *peerDevice {
	return &peerDevice{
		peerStore:    peerStore,
		DeviceUniqID: deviceUniqueID,
		Addrs:        NewSet[types.Address](nil),
		Sigpubkeys:   make(map[types.Address]*crypto.SigningPublicKey),
		Encpubkeys:   make(map[types.Address]*crypto.AsymEncPubkey),
		Stateuris:    NewSet[string](nil),
		Endpts:       make(map[PeerDialInfo]*peerEndpoint),
	}
}

func (pd *peerDevice) ensureEndpoint(dialInfo PeerDialInfo) *peerEndpoint {
	e, exists := pd.Endpts[dialInfo]
	if exists {
		return e
	}
	e = &peerEndpoint{
		peerDevice: pd,
		Dialinfo:   dialInfo,
		backoff:    utils.ExponentialBackoff{Min: 3 * time.Second, Max: 30 * time.Second},
	}
	pd.Endpts[dialInfo] = e
	return e
}

func (pd *peerDevice) Addresses() Set[types.Address] {
	pd.rlock()
	defer pd.runlock()
	return pd.Addrs
}

func (pd *peerDevice) PublicKeys(addr types.Address) (*crypto.SigningPublicKey, *crypto.AsymEncPubkey) {
	pd.rlock()
	defer pd.runlock()
	return pd.Sigpubkeys[addr], pd.Encpubkeys[addr]
}

func (pd *peerDevice) DeviceUniqueID() string {
	pd.rlock()
	defer pd.runlock()
	return pd.DeviceUniqID
}

func (pd *peerDevice) SetDeviceUniqueID(id string) {
	if id == "" {
		return
	}
	pd.lock()
	defer pd.unlock()
	pd.DeviceUniqID = id
}

func (pd *peerDevice) StateURIs() Set[string] {
	pd.rlock()
	defer pd.runlock()
	return pd.Stateuris.Copy()
}

func (pd *peerDevice) AddStateURI(stateURI string) {
	pd.lock()
	defer pd.unlock()
	pd.Stateuris.Add(stateURI)
}

func (pd *peerDevice) RemoveStateURI(stateURI string) {
	pd.lock()
	defer pd.unlock()
	pd.Stateuris.Remove(stateURI)
}

func (pd *peerDevice) LastContact() time.Time {
	pd.rlock()
	defer pd.runlock()
	var lastContact time.Time
	for _, e := range pd.Endpts {
		if time.Time(e.Lastcontact).After(lastContact) {
			lastContact = time.Time(e.Lastcontact)
		}
	}
	return lastContact
}

func (pd *peerDevice) LastFailure() time.Time {
	pd.rlock()
	defer pd.runlock()
	var lastFailure time.Time
	for _, e := range pd.Endpts {
		if time.Time(e.Lastfailure).After(lastFailure) {
			lastFailure = time.Time(e.Lastfailure)
		}
	}
	return lastFailure
}

func (pd *peerDevice) Failures() uint64 {
	pd.rlock()
	defer pd.runlock()
	var failures uint64
	for _, e := range pd.Endpts {
		failures += e.Fails
	}
	return failures
}

func (pd *peerDevice) Ready() bool {
	pd.rlock()
	defer pd.runlock()
	for _, e := range pd.Endpts {
		ready, _ := e.backoff.Ready()
		if ready {
			return true
		}
	}
	return false
}

func (pd *peerDevice) RemainingBackoff() time.Duration {
	pd.rlock()
	defer pd.runlock()
	minRemaining := time.Duration(math.MaxInt64)
	for _, e := range pd.Endpts {
		_, remaining := e.backoff.Ready()
		if remaining < minRemaining {
			minRemaining = remaining
		}
	}
	return minRemaining
}

func (pd *peerDevice) Endpoints() map[PeerDialInfo]PeerEndpoint {
	pd.rlock()
	defer pd.runlock()
	peerEndpoints := make(map[PeerDialInfo]PeerEndpoint, len(pd.Endpts))
	for dialInfo, e := range pd.Endpts {
		peerEndpoints[dialInfo] = e
	}
	return peerEndpoints
}

func (pd *peerDevice) Endpoint(dialInfo PeerDialInfo) (PeerEndpoint, bool) {
	pd.rlock()
	defer pd.runlock()
	e, exists := pd.Endpts[dialInfo]
	return e, exists
}

func (pd *peerDevice) MarshalStateBytes() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(pd)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (pd *peerDevice) UnmarshalStateBytes(bs []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(bs))
	return dec.Decode(pd)
}

func (pd *peerDevice) lock()    { pd.peerStore.muPeers.Lock() }
func (pd *peerDevice) unlock()  { pd.peerStore.muPeers.Unlock() }
func (pd *peerDevice) rlock()   { pd.peerStore.muPeers.RLock() }
func (pd *peerDevice) runlock() { pd.peerStore.muPeers.RUnlock() }

type PeerEndpoint interface {
	PeerDevice
	DialInfo() PeerDialInfo
	Dialable() bool
	UpdateConnStats(success bool)
	LastContact() time.Time
	LastFailure() time.Time
	Failures() uint64
	Ready() bool
	RemainingBackoff() time.Duration
}

type peerEndpoint struct {
	*peerDevice `tree:"-"`
	Dialinfo    PeerDialInfo             `tree:"dialInfo"`
	Lastcontact time.Time                `tree:"lastContact"`
	Lastfailure time.Time                `tree:"lastFailure"`
	Fails       uint64                   `tree:"failures"`
	backoff     utils.ExponentialBackoff `tree:"-"`
}

var _ PeerEndpoint = (*peerEndpoint)(nil)

func (e *peerEndpoint) Dialable() bool {
	e.rlock()
	defer e.runlock()
	return e.Dialinfo.DialAddr != ""
}

func (e *peerEndpoint) DialInfo() PeerDialInfo {
	e.rlock()
	defer e.runlock()
	return e.Dialinfo
}

func (e *peerEndpoint) UpdateConnStats(success bool) {
	e.lock()
	defer e.unlock()

	now := time.Now()
	if success {
		e.Lastcontact = now
		e.Fails = 0
	} else {
		e.Lastfailure = now
		e.Fails++
		e.backoff.Next()
	}
}

func (e *peerEndpoint) LastContact() time.Time {
	e.rlock()
	defer e.runlock()
	return time.Time(e.Lastcontact)
}

func (e *peerEndpoint) LastFailure() time.Time {
	e.rlock()
	defer e.runlock()
	return time.Time(e.Lastfailure)
}

func (e *peerEndpoint) Failures() uint64 {
	e.rlock()
	defer e.runlock()
	return e.Fails
}

// @@TODO: this should be configurable and probably not handled here
func (e *peerEndpoint) Ready() bool {
	e.rlock()
	defer e.runlock()
	ready, _ := e.backoff.Ready()
	return ready
}

func (e *peerEndpoint) RemainingBackoff() time.Duration {
	e.rlock()
	defer e.runlock()
	_, remaining := e.backoff.Ready()
	return remaining
}
