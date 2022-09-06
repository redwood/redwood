package prototree

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"redwood.dev/identity"
	"redwood.dev/state"
	"redwood.dev/swarm/protohush"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
	. "redwood.dev/utils/generics"
	"redwood.dev/vault"
)

type VaultManager struct {
	*vault.ClientManager
	store         Store
	keyStore      identity.KeyStore
	controllerHub tree.ControllerHub
	acl           ACL

	encryptedStateURIs *utils.Mailbox[vaultItem[protohush.GroupMessage]]
	encryptedTxs       *utils.Mailbox[vaultItem[protohush.GroupMessage]]
}

type vaultItem[T any] struct {
	vault.ClientItem
	Item T
}

func NewVaultManager(store Store, keyStore identity.KeyStore, controllerHub tree.ControllerHub, acl ACL, syncInterval time.Duration) *VaultManager {
	vm := &VaultManager{
		store:              store,
		keyStore:           keyStore,
		controllerHub:      controllerHub,
		acl:                acl,
		encryptedStateURIs: utils.NewMailbox[vaultItem[protohush.GroupMessage]](1000),
		encryptedTxs:       utils.NewMailbox[vaultItem[protohush.GroupMessage]](1000),
	}
	vm.ClientManager = vault.NewClientManager(ProtocolName, store, keyStore, syncInterval, vm.syncWithVault)
	return vm
}

func (vm *VaultManager) EncryptedStateURIs() *utils.Mailbox[vaultItem[protohush.GroupMessage]] {
	return vm.encryptedStateURIs
}

func (vm *VaultManager) EncryptedTxs() *utils.Mailbox[vaultItem[protohush.GroupMessage]] {
	return vm.encryptedTxs
}

func (vm *VaultManager) syncWithVault(ctx context.Context, client *vault.Client) error {
	vm.Infow("syncing with vault", "host", client.Host())

	// Fetch remote encrypted stateURIs
	addrs, err := vm.keyStore.Addresses()
	if err != nil {
		return err
	}
	for addr := range addrs {
		ch, err := client.FetchLatestFromCollection(ctx, collectionIDForEncryptedStateURIs(addr))
		if err != nil {
			vm.Errorw("failed to fetch encrypted state uris from mailbox", "address", addr, "host", client.Host(), "err", err)
			continue
		}
		vm.deliverEncryptedStateURIs(ctx, ch, client.Host())
	}

	// Store local encrypted stateURIs
	stateURIs, err := vm.controllerHub.StateURIsWithData()
	if err != nil {
		return err
	}
	for stateURI := range stateURIs {
		if vm.acl.TypeOf(stateURI) != StateURIType_Private {
			continue
		}

		encryptedStateURI, err := vm.store.EncryptedStateURI(tree.StateURI(stateURI))
		if err != nil {
			vm.Errorw("could not fetch encrypted state uri", "stateuri", stateURI, "err", err)
			continue
		}

		members, err := vm.acl.MembersOf(stateURI)
		if err != nil {
			vm.Errorw("failed to fetch members of state uri", "stateuri", stateURI, "err", err)
			continue
		}
		err = vm.addEncryptedStateURIForUsers(ctx, tree.StateURI(stateURI), members, encryptedStateURI, client)
		if err != nil {
			vm.Errorw("failed to add encrypted state uri", "stateuri", stateURI, "host", client.Host(), "err", err)
			continue
		}
	}

	// Fetch remote encrypted txs
	for stateURI := range vm.store.SubscribedStateURIs() {
		if vm.acl.TypeOf(string(stateURI)) != StateURIType_Private {
			continue
		}

		ch, err := client.FetchLatestFromCollection(ctx, collectionIDForEncryptedTxs(tree.StateURI(stateURI)))
		if err != nil {
			vm.Errorw("failed to fetch encrypted txs", "stateuri", stateURI, "address", client.Identity().Address(), "host", client.Host(), "err", err)
			continue
		}
		vm.deliverEncryptedTxs(ctx, ch, client.Host())
	}

	// Store local encrypted txs
	iter := vm.store.AllEncryptedTxs()
	defer iter.Close()

	for iter.Rewind(); iter.Valid(); iter.Next() {
		stateURI, txID, etx := iter.Current()

		err := vm.addEncryptedTx(ctx, stateURI, txID, etx, client)
		if err != nil {
			vm.Errorw("failed to add encrypted tx", "host", client.Host(), "stateuri", stateURI, "txID", txID, "err", err)
			continue
		}
	}
	return nil
}

func (vm *VaultManager) deliverEncryptedStateURIs(ctx context.Context, ch <-chan vault.ClientItem, host string) {
	for {
		select {
		case <-ctx.Done():
			return

		case item, open := <-ch:
			if !open {
				return
			}

			func() {
				defer item.Close()

				bs, err := ioutil.ReadAll(item)
				if err != nil {
					vm.Errorw("failed to read encrypted state uri from vault", "host", host, "err", err)
					return
				}

				var msg protohush.GroupMessage
				err = msg.Unmarshal(bs)
				if err != nil {
					vm.Errorw("failed to unmarshal encrypted state uri from vault", "host", host, "err", err)
					return
				}

				vm.encryptedStateURIs.Deliver(vaultItem[protohush.GroupMessage]{item, msg})
			}()
		}
	}
}

func (vm *VaultManager) deliverEncryptedTxs(ctx context.Context, ch <-chan vault.ClientItem, host string) {
	for {
		select {
		case <-ctx.Done():
			return

		case item, open := <-ch:
			if !open {
				return
			}

			func() {
				defer item.Close()

				bs, err := ioutil.ReadAll(item)
				if err != nil {
					vm.Errorw("failed to read encrypted tx from vault", "host", host, "err", err)
					return
				}

				var msg protohush.GroupMessage
				err = msg.Unmarshal(bs)
				if err != nil {
					vm.Errorw("failed to unmarshal encrypted tx from vault", "host", host, "err", err)
					return
				}

				vm.encryptedTxs.Deliver(vaultItem[protohush.GroupMessage]{item, msg})
			}()
		}
	}
}

func (vm *VaultManager) AddEncryptedStateURIForUsers(ctx context.Context, stateURI tree.StateURI, users Set[types.Address], msg protohush.GroupMessage) error {
	return vm.WithAllVaultClients(ctx, "add state uri to user mailboxes", func(ctx context.Context, client *vault.Client) error {
		return vm.addEncryptedStateURIForUsers(ctx, stateURI, users, msg, client)
	})
}

func (vm *VaultManager) addEncryptedStateURIForUsers(ctx context.Context, stateURI tree.StateURI, users Set[types.Address], msg protohush.GroupMessage, client *vault.Client) error {
	for user := range users {
		bs, err := msg.Marshal()
		if err != nil {
			vm.Errorw("failed to marshal state uri for user", "stateuri", stateURI, "user", user, "host", client.Host(), "err", err)
			return nil
		}
		err = client.Store(ctx, collectionIDForEncryptedStateURIs(user), types.HashBytes([]byte(stateURI)).Hex(), bytes.NewReader(bs))
		if err != nil {
			vm.Errorw("failed to store encrypted state uri for user", "stateuri", stateURI, "user", user, "host", client.Host(), "err", err)
			return nil
		}
	}
	return nil
}

func (vm *VaultManager) AddEncryptedTx(ctx context.Context, stateURI tree.StateURI, txID state.Version, msg protohush.GroupMessage) error {
	return vm.WithAllVaultClients(ctx, fmt.Sprintf("add encrypted tx %v %v", stateURI, txID), func(ctx context.Context, client *vault.Client) error {
		return vm.addEncryptedTx(ctx, stateURI, txID, msg, client)
	})
}

func (vm *VaultManager) addEncryptedTx(ctx context.Context, stateURI tree.StateURI, txID state.Version, msg protohush.GroupMessage, client *vault.Client) error {
	bs, err := msg.Marshal()
	if err != nil {
		vm.Errorw("failed to marshal tx for user", "stateuri", stateURI, "tx", txID, "host", client.Host(), "err", err)
		return nil
	}
	err = client.Store(ctx, collectionIDForEncryptedTxs(stateURI), txID.Hex(), bytes.NewReader(bs))
	if err != nil {
		vm.Errorw("failed to store tx for user", "stateuri", stateURI, "tx", txID, "host", client.Host(), "err", err)
		return nil
	}
	return nil
}

func collectionIDForTxs() string {
	return "txs"
}

func collectionIDForEncryptedStateURIs(userAddr types.Address) string {
	return "mailbox-" + types.HashBytes(userAddr[:]).Hex()
}

func collectionIDForEncryptedTxs(stateURI tree.StateURI) string {
	return "txs-" + types.HashBytes([]byte(stateURI)).Hex()
}
