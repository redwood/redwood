package prototree

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/swarm/protohush"
	"redwood.dev/tree"
	"redwood.dev/utils"
	. "redwood.dev/utils/generics"
)

//go:generate mockery --name Store --output ./mocks/ --case=underscore
type Store interface {
	SubscribedStateURIs() Set[tree.StateURI]
	AddSubscribedStateURI(stateURI tree.StateURI) error
	RemoveSubscribedStateURI(stateURI tree.StateURI) error
	OnNewSubscribedStateURI(handler func(stateURI tree.StateURI)) (unsubscribe func())

	MaxPeersPerSubscription() uint64
	SetMaxPeersPerSubscription(max uint64) error

	TxSeenByPeer(deviceUniqueID string, stateURI tree.StateURI, txID state.Version) bool
	MarkTxSeenByPeer(deviceUniqueID string, stateURI tree.StateURI, txID state.Version) error
	PruneTxSeenRecordsOlderThan(threshold time.Duration) error

	OpaqueEncryptedTx(messageID string) (protohush.GroupMessage, error)
	OpaqueEncryptedTxs() map[string]protohush.GroupMessage
	SaveOpaqueEncryptedTx(messageID string, encryptedTx protohush.GroupMessage) error
	DeleteOpaqueEncryptedTx(messageID string) error

	AllEncryptedStateURIs() *encryptedStateURIIterator
	EncryptedStateURI(stateURI tree.StateURI) (protohush.GroupMessage, error)
	SaveEncryptedStateURI(stateURI tree.StateURI, msg protohush.GroupMessage) error

	AllEncryptedTxs() *encryptedTxIterator
	EncryptedTx(stateURI tree.StateURI, txID state.Version) (protohush.GroupMessage, error)
	SaveEncryptedTx(stateURI tree.StateURI, txID state.Version, msg protohush.GroupMessage) error

	Vaults() Set[string]
	AddVault(host string) error
	RemoveVault(host string) error
	LatestMtimeForVaultAndCollection(host, collectionID string) time.Time
	MaybeSaveLatestMtimeForVaultAndCollection(host, collectionID string, mtime time.Time) error

	DebugPrint()
}

type store struct {
	process.Process
	log.Logger

	db *state.DBTree

	data   storeData
	dataMu sync.RWMutex

	subscribedStateURIListeners SyncSet[*subscribedStateURIListener]
}

type subscribedStateURIListener struct {
	handler func(stateURI tree.StateURI)
}

type storeData struct {
	SubscribedStateURIs              Set[tree.StateURI]                                         `tree:"subscribedStateURIs"`
	MaxPeersPerSubscription          uint64                                                     `tree:"maxPeersPerSubscription"`
	TxsSeenByPeers                   map[string]map[tree.StateURI]map[state.Version]uint64      `tree:"txsSeenByPeers"`
	OpaqueEncryptedTxs               map[string]protohush.GroupMessage                          `tree:"opaqueEncryptedTxs"`
	LatestMtimeForVaultAndCollection map[string]map[string]uint64                               `tree:"latestMtimeForVaultAndCollection"`
	EncryptedTxs                     map[tree.StateURI]map[state.Version]protohush.GroupMessage `tree:"encryptedTxs"`
	EncryptedStateURIs               map[tree.StateURI]protohush.GroupMessage                   `tree:"encryptedStateURIs"`
	Vaults                           Set[string]                                                `tree:"vaults"`
}

var storeRootKeypath = state.Keypath("prototree")

func NewStore(db *state.DBTree) (*store, error) {
	s := &store{
		Process:                     *process.New("prototree store"),
		Logger:                      log.NewLogger("prototree"),
		db:                          db,
		subscribedStateURIListeners: NewSyncSet[*subscribedStateURIListener](nil),
	}
	s.Infof("opening prototree store at %v", db.Filename())
	err := s.loadData()
	return s, err
}

func (s *store) Start() error {
	err := s.Process.Start()
	if err != nil {
		return err
	}

	pruneTxsTask := NewPruneTxsTask(5*time.Minute, s)
	err = s.Process.SpawnChild(nil, pruneTxsTask)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) loadData() error {
	node := s.db.State(false)
	defer node.Close()

	err := node.NodeAt(storeRootKeypath, nil).Scan(&s.data)
	if errors.Cause(err) == errors.Err404 {
		// do nothing
	} else if err != nil {
		return err
	}

	for encodedHost := range s.data.Vaults {
		s.data.Vaults.Remove(encodedHost)
		host, err := url.QueryUnescape(encodedHost)
		if err != nil {
			continue
		}
		s.data.Vaults.Add(host)
	}

	return nil
}

func (s *store) SubscribedStateURIs() Set[tree.StateURI] {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()
	return s.data.SubscribedStateURIs.Copy()
}

func (s *store) AddSubscribedStateURI(stateURI tree.StateURI) error {
	s.subscribedStateURIListeners.ForEach(func(listener *subscribedStateURIListener) {
		listener.handler(stateURI)
	})

	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	s.data.SubscribedStateURIs.Add(stateURI)

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(s.keypathForSubscribedStateURI(stateURI), nil, true)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) RemoveSubscribedStateURI(stateURI tree.StateURI) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	s.data.SubscribedStateURIs.Remove(stateURI)

	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(s.keypathForSubscribedStateURI(stateURI), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) OnNewSubscribedStateURI(handler func(stateURI tree.StateURI)) (unsubscribe func()) {
	listener := &subscribedStateURIListener{handler: handler}
	s.subscribedStateURIListeners.Add(listener)
	return func() { s.subscribedStateURIListeners.Remove(listener) }
}

func (s *store) MaxPeersPerSubscription() uint64 {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()
	return s.data.MaxPeersPerSubscription
}

func (s *store) SetMaxPeersPerSubscription(max uint64) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	s.data.MaxPeersPerSubscription = max

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(s.keypathForMaxPeersPerSubscription(), nil, max)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) TxSeenByPeer(deviceUniqueID string, stateURI tree.StateURI, txID state.Version) bool {
	if len(deviceUniqueID) == 0 {
		return false
	}

	s.dataMu.RLock()
	defer s.dataMu.RUnlock()

	if _, exists := s.data.TxsSeenByPeers[deviceUniqueID]; !exists {
		return false
	}
	if _, exists := s.data.TxsSeenByPeers[deviceUniqueID][stateURI]; !exists {
		return false
	}
	_, exists := s.data.TxsSeenByPeers[deviceUniqueID][stateURI][txID]
	return exists
}

func (s *store) MarkTxSeenByPeer(deviceUniqueID string, stateURI tree.StateURI, txID state.Version) error {
	if len(deviceUniqueID) == 0 {
		return nil
	}

	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	now := uint64(time.Now().UTC().Unix())

	if _, exists := s.data.TxsSeenByPeers[deviceUniqueID]; !exists {
		s.data.TxsSeenByPeers[deviceUniqueID] = make(map[tree.StateURI]map[state.Version]uint64)
	}
	if _, exists := s.data.TxsSeenByPeers[deviceUniqueID][stateURI]; !exists {
		s.data.TxsSeenByPeers[deviceUniqueID][stateURI] = make(map[state.Version]uint64)
	}
	s.data.TxsSeenByPeers[deviceUniqueID][stateURI][txID] = now

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(s.keypathForTxSeenByPeer(deviceUniqueID, stateURI, txID), nil, now)
	if err != nil {
		return err
	}

	return node.Save()
}

type pruneTxsTask struct {
	process.PeriodicTask
	log.Logger

	interval time.Duration
	store    Store
}

func NewPruneTxsTask(
	interval time.Duration,
	store Store,
) *pruneTxsTask {
	t := &pruneTxsTask{
		Logger: log.NewLogger("prototree store"),
		store:  store,
	}
	t.PeriodicTask = *process.NewPeriodicTask("PruneTxsTask", utils.NewStaticTicker(interval), t.pruneTxs)
	return t
}

func (t *pruneTxsTask) pruneTxs(ctx context.Context) {
	err := t.store.PruneTxSeenRecordsOlderThan(t.interval)
	if err != nil {
		t.Errorf("while pruning prototree store txs: %v", err)
	}
}

func (s *store) PruneTxSeenRecordsOlderThan(threshold time.Duration) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	node := s.db.State(true)
	defer node.Close()

	s.Debugf("pruning prototree store txs")

	for deviceUniqueID, y := range s.data.TxsSeenByPeers {
		for stateURI, z := range y {
			for txID, whenSeen := range z {
				t := time.Unix(int64(whenSeen), 0)
				if t.Add(threshold).Before(time.Now().UTC()) {
					delete(s.data.TxsSeenByPeers[deviceUniqueID][stateURI], txID)

					err := node.Delete(s.keypathForTxSeenByPeer(deviceUniqueID, stateURI, txID), nil)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return node.Save()
}

func (s *store) OpaqueEncryptedTxs() map[string]protohush.GroupMessage {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()

	m := make(map[string]protohush.GroupMessage, len(s.data.OpaqueEncryptedTxs))
	for id, tx := range s.data.OpaqueEncryptedTxs {
		m[id] = tx.Copy()
	}
	return m
}

func (s *store) LatestMtimeForVaultAndCollection(vaultHost, collectionID string) time.Time {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()

	collections, ok := s.data.LatestMtimeForVaultAndCollection[vaultHost]
	if !ok {
		return time.Time{}
	}
	return time.Unix(int64(collections[collectionID]), 0)
}

func (s *store) MaybeSaveLatestMtimeForVaultAndCollection(vaultHost, collectionID string, mtime time.Time) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	_, ok := s.data.LatestMtimeForVaultAndCollection[vaultHost]
	if !ok {
		s.data.LatestMtimeForVaultAndCollection[vaultHost] = make(map[string]uint64)
	}

	if time.Unix(int64(s.data.LatestMtimeForVaultAndCollection[vaultHost][collectionID]), 0).After(mtime) {
		return nil
	}
	s.data.LatestMtimeForVaultAndCollection[vaultHost][collectionID] = uint64(mtime.UTC().Unix())

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(s.keypathForLatestMtimeForVaultAndCollection(vaultHost, collectionID), nil, mtime)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) OpaqueEncryptedTx(messageID string) (protohush.GroupMessage, error) {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()

	tx, exists := s.data.OpaqueEncryptedTxs[messageID]
	if !exists {
		return protohush.GroupMessage{}, errors.Err404
	}
	return tx, nil
}

func (s *store) SaveOpaqueEncryptedTx(messageID string, encryptedTx protohush.GroupMessage) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(s.keypathForOpaqueEncryptedTx(messageID), nil, encryptedTx)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) DeleteOpaqueEncryptedTx(messageID string) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	delete(s.data.OpaqueEncryptedTxs, messageID)

	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(s.keypathForOpaqueEncryptedTx(messageID), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

type encryptedTxIterator struct {
	ch        chan *encryptedTxIteratorTuple
	chCancel  chan struct{}
	current   *encryptedTxIteratorTuple
	closeOnce *sync.Once
}

type encryptedTxIteratorTuple struct {
	stateURI tree.StateURI
	id       state.Version
	tx       protohush.GroupMessage
}

func (iter *encryptedTxIterator) Rewind() {
	iter.Next()
}

func (iter *encryptedTxIterator) Next() {
	select {
	case <-iter.chCancel:
		return
	case tpl, open := <-iter.ch:
		if !open {
			iter.current = nil
			return
		}
		iter.current = tpl
	}
}

func (iter *encryptedTxIterator) Valid() bool {
	return iter.current != nil
}

func (iter *encryptedTxIterator) Close() {
	iter.closeOnce.Do(func() {
		iter.current = nil
		close(iter.chCancel)
	})
}

func (iter *encryptedTxIterator) Current() (tree.StateURI, state.Version, protohush.GroupMessage) {
	if iter.current == nil {
		panic("invariant violation: iterator misuse")
	}
	return iter.current.stateURI, iter.current.id, iter.current.tx
}

type encryptedStateURIIterator struct {
	ch        chan *encryptedStateURIIteratorTuple
	chCancel  chan struct{}
	current   *encryptedStateURIIteratorTuple
	closeOnce *sync.Once
}

type encryptedStateURIIteratorTuple struct {
	stateURI          tree.StateURI
	encryptedStateURI protohush.GroupMessage
}

func (iter *encryptedStateURIIterator) Rewind() {
	iter.Next()
}

func (iter *encryptedStateURIIterator) Next() {
	select {
	case <-iter.chCancel:
		return
	case tpl, open := <-iter.ch:
		if !open {
			iter.current = nil
			return
		}
		iter.current = tpl
	}
}

func (iter *encryptedStateURIIterator) Valid() bool {
	return iter.current != nil
}

func (iter *encryptedStateURIIterator) Close() {
	iter.closeOnce.Do(func() {
		iter.current = nil
		close(iter.chCancel)
	})
}

func (iter *encryptedStateURIIterator) Current() (tree.StateURI, protohush.GroupMessage) {
	if iter.current == nil {
		panic("invariant violation: iterator misuse")
	}
	return iter.current.stateURI, iter.current.encryptedStateURI
}

func (s *store) AllEncryptedStateURIs() *encryptedStateURIIterator {
	iter := &encryptedStateURIIterator{
		ch:        make(chan *encryptedStateURIIteratorTuple),
		chCancel:  make(chan struct{}),
		closeOnce: &sync.Once{},
	}

	var stateURIs []tree.StateURI
	func() {
		s.dataMu.RLock()
		defer s.dataMu.RUnlock()
		stateURIs = Keys(s.data.EncryptedStateURIs)
	}()
	if len(stateURIs) == 0 {
		iter.Close()
		return iter
	}

	go func() {
		defer close(iter.ch)

		for _, stateURI := range stateURIs {
			encryptedStateURI, err := s.EncryptedStateURI(stateURI)
			if err != nil {
				continue
			}

			select {
			case iter.ch <- &encryptedStateURIIteratorTuple{stateURI, encryptedStateURI}:
			case <-iter.chCancel:
				return
			}
		}
	}()

	return iter
}

func (s *store) EncryptedStateURI(stateURI tree.StateURI) (protohush.GroupMessage, error) {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()

	msg, exists := s.data.EncryptedStateURIs[stateURI]
	if !exists {
		return protohush.GroupMessage{}, errors.Err404
	}
	return msg, nil
}

func (s *store) SaveEncryptedStateURI(stateURI tree.StateURI, msg protohush.GroupMessage) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	s.data.EncryptedStateURIs[stateURI] = msg

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(s.encryptedStateURIKeypathFor(stateURI), nil, msg)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) AllEncryptedTxs() *encryptedTxIterator {
	iter := &encryptedTxIterator{
		ch:        make(chan *encryptedTxIteratorTuple),
		chCancel:  make(chan struct{}),
		closeOnce: &sync.Once{},
	}

	var stateURIs []tree.StateURI
	func() {
		s.dataMu.RLock()
		defer s.dataMu.RUnlock()
		stateURIs = Keys(s.data.EncryptedTxs)
	}()
	if len(stateURIs) == 0 {
		iter.Close()
		return iter
	}

	go func() {
		defer close(iter.ch)

		for _, stateURI := range stateURIs {
			var ids []state.Version
			func() {
				s.dataMu.RLock()
				defer s.dataMu.RUnlock()
				ids = Keys(s.data.EncryptedTxs[stateURI])
			}()

			for _, id := range ids {
				var tx protohush.GroupMessage
				func() {
					s.dataMu.RLock()
					defer s.dataMu.RUnlock()
					tx = s.data.EncryptedTxs[stateURI][id].Copy()
				}()

				select {
				case iter.ch <- &encryptedTxIteratorTuple{stateURI, id, tx}:
				case <-iter.chCancel:
					return
				}
			}
		}
	}()

	return iter
}

func (s *store) EncryptedTx(stateURI tree.StateURI, txID state.Version) (protohush.GroupMessage, error) {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()

	_, exists := s.data.EncryptedTxs[tree.StateURI(stateURI)]
	if !exists {
		return protohush.GroupMessage{}, errors.Err404
	}
	etx, exists := s.data.EncryptedTxs[tree.StateURI(stateURI)][txID]
	if !exists {
		return protohush.GroupMessage{}, errors.Err404
	}
	return etx, nil
}

func (s *store) SaveEncryptedTx(stateURI tree.StateURI, txID state.Version, etx protohush.GroupMessage) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	_, exists := s.data.EncryptedTxs[tree.StateURI(stateURI)]
	if !exists {
		s.data.EncryptedTxs[tree.StateURI(stateURI)] = make(map[state.Version]protohush.GroupMessage)
	}
	s.data.EncryptedTxs[tree.StateURI(stateURI)][txID] = etx

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(s.keypathForEncryptedTx(tree.StateURI(stateURI), txID), nil, etx)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) Vaults() Set[string] {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()
	return s.data.Vaults.Copy()
}

func (s *store) AddVault(host string) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	s.data.Vaults.Add(host)

	node := s.db.State(true)
	defer node.Close()

	keypath := s.vaultsKeypathForHost(host)

	err := node.Set(keypath, nil, true)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) RemoveVault(host string) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	s.data.Vaults.Remove(host)

	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(s.vaultsKeypathForHost(host), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) keypathForSubscribedStateURI(stateURI tree.StateURI) state.Keypath {
	return storeRootKeypath.Pushs("subscribedStateURIs").Pushs(url.QueryEscape(string(stateURI)))
}

func (s *store) keypathForMaxPeersPerSubscription() state.Keypath {
	return storeRootKeypath.Pushs("maxPeersPerSubscription")
}

func (s *store) keypathForTxSeenByPeer(deviceUniqueID string, stateURI tree.StateURI, txID state.Version) state.Keypath {
	return storeRootKeypath.Pushs("txsSeenByPeers").Pushs(deviceUniqueID).Pushs(url.QueryEscape(string(stateURI))).Pushs(txID.Hex())
}

func (s *store) vaultsKeypath() state.Keypath {
	return storeRootKeypath.Copy().Pushs("vaults")
}

func (s *store) vaultsKeypathForHost(host string) state.Keypath {
	return s.vaultsKeypath().Pushs(url.QueryEscape(host))
}

func (s *store) keypathForEncryptedTx(stateURI tree.StateURI, txID state.Version) state.Keypath {
	return storeRootKeypath.Copy().Pushs("encryptedTxs").Pushs(url.QueryEscape(string(stateURI))).Pushs(txID.Hex())
}

func (s *store) keypathForOpaqueEncryptedTx(messageID string) state.Keypath {
	return storeRootKeypath.Copy().Pushs("opaqueEncryptedTxs").Pushs(messageID)
}

func (s *store) encryptedStateURIKeypathFor(stateURI tree.StateURI) state.Keypath {
	return storeRootKeypath.Copy().Pushs("encryptedStateURIs").Pushs(url.QueryEscape(string(stateURI)))
}

func (s *store) keypathForLatestMtimeForVaultAndCollection(vaultHost, collectionID string) state.Keypath {
	return storeRootKeypath.Copy().Pushs("latestMtimeForVaultAndCollection").Pushs(url.QueryEscape(vaultHost)).Pushs(collectionID)
}

func (s *store) DebugPrint() {
	node := s.db.State(false)
	defer node.Close()
	node.NodeAt(storeRootKeypath, nil).DebugPrint(func(msg string, args ...interface{}) { fmt.Printf(msg, args...) }, true, 0)
}
