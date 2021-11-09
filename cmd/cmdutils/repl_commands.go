package cmdutils

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type REPLCommand struct {
	Command  string
	HelpText string
	Handler  func(args []string, app *App) error
}

var (
	CmdMnemonic = REPLCommand{
		"mnemonic",
		"show your identity's mnemonic",
		func(args []string, app *App) error {
			m, err := app.KeyStore.Mnemonic()
			if err != nil {
				return err
			}
			app.Debugf("mnemonic: %v", m)
			return nil
		},
	}

	CmdAddress = REPLCommand{
		"address",
		"show your address",
		func(args []string, app *App) error {
			identity, err := app.KeyStore.DefaultPublicIdentity()
			if err != nil {
				return err
			}
			app.Debugf("address: %v", identity.Address())
			return nil
		},
	}

	CmdLibp2pPeerID = REPLCommand{
		"libp2pid",
		"show your libp2p peer ID",
		func(args []string, app *App) error {
			if app.Libp2pTransport == nil {
				return errors.New("libp2p is disabled")
			}
			peerID := app.Libp2pTransport.Libp2pPeerID()
			app.Debugf("libp2p peer ID: %v", peerID)
			return nil
		},
	}

	CmdSubscribe = REPLCommand{
		"subscribe",
		"subscribe to a state URI",
		func(args []string, app *App) error {
			if len(args) < 1 {
				return errors.New("missing argument: state URI")
			}

			stateURI := args[0]

			sub, err := app.TreeProto.Subscribe(
				context.Background(),
				stateURI,
				prototree.SubscriptionType_Txs,
				nil,
				&prototree.FetchHistoryOpts{FromTxID: tree.GenesisTxID},
			)
			if err != nil {
				return err
			}
			sub.Close()
			return nil
		},
	}

	CmdStateURIs = REPLCommand{
		"stateuris",
		"list all known state URIs",
		func(args []string, app *App) error {
			stateURIs, err := app.ControllerHub.KnownStateURIs()
			if err != nil {
				return err
			}
			if len(stateURIs) == 0 {
				fmt.Println("no known state URIs")
			} else {
				for _, stateURI := range stateURIs {
					fmt.Println("- ", stateURI)
				}
			}
			return nil
		},
	}

	CmdGetState = REPLCommand{
		"state",
		"print the current state tree for a state URI",
		func(args []string, app *App) error {
			if len(args) < 1 {
				return errors.New("missing argument: state URI")
			}

			stateURI := args[0]
			node, err := app.ControllerHub.StateAtVersion(stateURI, nil)
			if err != nil {
				return err
			}
			var keypath state.Keypath
			var rng *state.Range
			if len(args) > 1 {
				_, keypath, rng, err = tree.ParsePatchPath([]byte(args[1]))
				if err != nil {
					return err
				}
			}
			app.Debugf("stateURI: %v / keypath: %v / range: %v", stateURI, keypath, rng)
			node = node.NodeAt(keypath, rng)
			node.DebugPrint(app.Debugf, false, 0)
			fmt.Println(utils.PrettyJSON(node))
			return nil
		},
	}

	CmdSetState = REPLCommand{
		"set",
		"set a keypath in a state tree",
		func(args []string, app *App) error {
			if len(args) < 3 {
				return errors.New("requires 3 arguments: set <state URI> <keypath> <JSON value>")
			}
			stateURI := args[0]
			keypath := state.Keypath(args[1])
			jsonVal := strings.Join(args[2:], " ")

			_, err := app.ControllerHub.StateAtVersion(stateURI, nil)
			var txID state.Version
			if errors.Cause(err) == tree.ErrNoController {
				txID = tree.GenesisTxID
			} else {
				txID = state.RandomVersion()
			}

			err = app.TreeProto.SendTx(context.TODO(), tree.Tx{
				ID:       txID,
				StateURI: stateURI,
				Patches: []tree.Patch{{
					Keypath:   keypath,
					ValueJSON: []byte(jsonVal),
				}},
			})
			return err
		},
	}

	CmdListTxs = REPLCommand{
		"txs",
		"list the txs for a given state URI",
		func(args []string, app *App) error {
			if len(args) < 1 {
				return errors.New("requires 1 arguments: txs <state URI>")
			}
			stateURI := args[0]

			iter := app.TxStore.AllTxsForStateURI(stateURI, tree.GenesisTxID)
			defer iter.Close()

			for {
				tx := iter.Next()
				if tx == nil {
					break
				}
				app.Debugf("- %v", tx.ID)
			}
			return nil
		},
	}

	CmdBlobs = REPLCommand{
		"blobs",
		"list all blobs",
		func(args []string, app *App) error {
			app.BlobStore.DebugPrint()

			contents, err := app.BlobStore.Contents()
			if err != nil {
				return err
			}

			var rows [][]string

			for blobHash, x := range contents {
				for chunkHash, have := range x {
					rows = append(rows, []string{blobHash.Hex(), chunkHash.Hex(), fmt.Sprintf("%v", have)})
				}
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
			table.SetCenterSeparator("|")
			table.SetRowLine(true)
			table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
			table.SetHeader([]string{"Blob", "Chunk", "Have"})
			table.AppendBulk(rows)
			table.Render()

			return nil
		},
	}

	CmdPeers = REPLCommand{
		"peers",
		"list all known peers",
		func(args []string, app *App) error {
			var full bool
			if len(args) > 0 {
				if args[0] == "full" {
					full = true
				} else {
					return errors.Errorf("unknown argument '%v'", args[0])
				}
			}

			fmtPeerRow := func(addr, duID, dialAddr string, lastContact, lastFailure time.Time, failures uint64, remainingBackoff time.Duration, stateURIs []string) []string {
				if !full && len(addr) > 10 {
					addr = addr[:4] + "..." + addr[len(addr)-4:]
				}
				if !full && len(dialAddr) > 30 {
					dialAddr = dialAddr[:30] + "..." + dialAddr[len(dialAddr)-6:]
				}
				lastContactStr := time.Now().Sub(lastContact).Round(1 * time.Second).String()
				if lastContact.IsZero() {
					lastContactStr = ""
				}
				lastFailureStr := time.Now().Sub(lastFailure).Round(1 * time.Second).String()
				if lastFailure.IsZero() {
					lastFailureStr = ""
				}
				failuresStr := fmt.Sprintf("%v", failures)
				remainingBackoffStr := remainingBackoff.Round(1 * time.Second).String()
				if remainingBackoff == 0 {
					remainingBackoffStr = ""
				}
				if !full && len(duID) > 4 {
					duID = duID[:4] + "..."
				}
				return []string{addr, duID, dialAddr, lastContactStr, lastFailureStr, failuresStr, remainingBackoffStr, fmt.Sprintf("%v", stateURIs)}
			}

			var data [][]string
			for _, peer := range app.PeerStore.Peers() {
				for _, e := range peer.Endpoints() {
					for _, addr := range peer.Addresses() {
						data = append(data, fmtPeerRow(addr.Hex(), e.DeviceUniqueID(), e.DialInfo().DialAddr, e.LastContact(), e.LastFailure(), e.Failures(), e.RemainingBackoff(), e.StateURIs().Slice()))
					}
					if len(e.Addresses()) == 0 {
						data = append(data, fmtPeerRow("?", e.DeviceUniqueID(), e.DialInfo().DialAddr, e.LastContact(), e.LastFailure(), e.Failures(), e.RemainingBackoff(), e.StateURIs().Slice()))
					}
				}
			}

			sort.Slice(data, func(i, j int) bool {
				cmp := strings.Compare(data[i][0], data[j][0])
				if cmp == 0 {
					return strings.Compare(data[i][1], data[j][1]) < 0
				} else {
					return cmp < 0
				}
			})

			table := tablewriter.NewWriter(os.Stdout)
			table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
			table.SetCenterSeparator("|")
			table.SetRowLine(true)
			table.SetHeader([]string{"Address", "DeviceID", "DialAddr", "LastContact", "LastFailure", "Failures", "Backoff", "StateURIs"})
			table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
			table.SetColumnColor(
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{},
				tablewriter.Colors{},
				tablewriter.Colors{},
				tablewriter.Colors{},
				tablewriter.Colors{},
				tablewriter.Colors{},
				tablewriter.Colors{},
			)
			table.AppendBulk(data)
			table.Render()

			return nil
		},
	}

	CmdAddPeer = REPLCommand{
		"addpeer",
		"add a peer",
		func(args []string, app *App) error {
			if len(args) < 2 {
				return errors.New("requires 2 arguments: addpeer <transport> <dial addr>")
			}
			app.PeerStore.AddDialInfo(swarm.PeerDialInfo{args[0], args[1]}, "")
			return nil
		},
	}

	CmdRemoveAllPeers = REPLCommand{
		"rmallpeers",
		"remove all peers",
		func(args []string, app *App) error {
			var toDelete []string
			for _, peer := range app.PeerStore.Peers() {
				toDelete = append(toDelete, peer.DeviceUniqueID())
			}
			app.PeerStore.RemovePeers(toDelete)
			return nil
		},
	}

	CmdRemoveUnverifiedPeers = REPLCommand{
		"rmunverifiedpeers",
		"remove peers who haven't been verified",
		func(args []string, app *App) error {
			var toDelete []string
			for _, peer := range app.PeerStore.Peers() {
				if len(peer.Addresses()) == 0 {
					toDelete = append(toDelete, peer.DeviceUniqueID())
				}
			}
			app.PeerStore.RemovePeers(toDelete)
			return nil
		},
	}

	CmdRemoveFailedPeers = REPLCommand{
		"rmfailedpeers",
		"remove peers with more than a certain number of failures",
		func(args []string, app *App) error {
			if len(args) < 1 {
				return errors.New("requires 1 argument: rmfailedpeers <number of failures>")
			}

			num, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrap(err, "bad argument")
			}

			var toDelete []string
			for _, peer := range app.PeerStore.Peers() {
				if peer.Failures() > uint64(num) {
					toDelete = append(toDelete, peer.DeviceUniqueID())
				}
			}
			app.PeerStore.RemovePeers(toDelete)
			return nil
		},
	}

	CmdHushSendIndividualMessage = REPLCommand{
		"hushmsg",
		"send a 1:1 Hush message",
		func(args []string, app *App) error {
			if len(args) < 2 {
				return errors.New("requires 2 arguments: hushmsg <recipient address> <message>")
			}

			recipient, err := types.AddressFromHex(args[0])
			if err != nil {
				return err
			}
			msg := strings.Join(args[1:], " ")

			return app.HushProto.EncryptIndividualMessage("foo", recipient, []byte(msg))
		},
	}

	CmdHushSendGroupMessage = REPLCommand{
		"hushgroup",
		"send a group Hush message",
		func(args []string, app *App) error {
			if len(args) < 2 {
				return errors.New("requires 2 arguments: hushmsg <comma-separated recipient addresses> <message>")
			}

			addrStrs := strings.Split(args[0], ",")
			var recipients []types.Address
			for _, s := range addrStrs {
				addr, err := types.AddressFromHex(s)
				if err != nil {
					return err
				}
				recipients = append(recipients, addr)
			}

			msg := strings.Join(args[1:], " ")

			id := types.RandomID()
			return app.HushProto.EncryptGroupMessage("foo", id.Hex(), recipients, []byte(msg))
		},
	}

	CmdHushStoreDebugPrint = REPLCommand{
		"hushstore",
		"print the contents of the protohush store",
		func(args []string, app *App) error {
			app.HushProtoStore.DebugPrint()
			return nil
		},
	}

	CmdTreeStoreDebugPrint = REPLCommand{
		"treestore",
		"print the contents of the prototree store",
		func(args []string, app *App) error {
			app.TreeProtoStore.DebugPrint()
			return nil
		},
	}

	CmdPeerStoreDebugPrint = REPLCommand{
		"peerstore",
		"print the contents of the peer store",
		func(args []string, app *App) error {
			app.PeerStore.DebugPrint()
			return nil
		},
	}

	CmdProcessTree = REPLCommand{
		Command:  "ps",
		HelpText: "display the current process tree",
		Handler: func(args []string, app *App) error {
			app.Infof(0, "processes:\n%v", utils.PrettyJSON(app.ProcessTree()))
			return nil
		},
	}
)
