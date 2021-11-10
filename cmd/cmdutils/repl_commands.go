package cmdutils

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/logrusorgru/aurora/v3"
	"github.com/olekukonko/tablewriter"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type REPLHandler interface {
	Handle(args []string, app *App) error
	Help() string
}

type REPLCommand struct {
	HelpText    string
	Subcommands REPLCommands
	Handler     func(args []string, app *App) error
}

func (c REPLCommand) Handle(cmdParts []string, args []string, app *App) error {
	if len(c.Subcommands) > 0 {
		if len(args) == 0 {
			return ErrShowHelp
		}
		return c.Subcommands.Handle(cmdParts, args, app)
	}
	return c.Handler(args, app)
}

func (c REPLCommand) Help() string {
	txt := c.HelpText + "\n"
	if len(c.Subcommands) > 0 {
		txt += fmt.Sprintf("\n%v %v", aurora.Bold(aurora.White("COMMANDS")), c.Subcommands.Help())
	}
	return txt
}

type REPLCommands map[string]REPLCommand

func (c REPLCommands) Handle(cmdParts []string, args []string, app *App) error {
	cmd, exists := c[args[0]]
	if !exists {
		fmt.Println("unknown command")
		return ErrShowHelp
	}
	fullCmd := append(cmdParts, args[0])
	err := cmd.Handle(fullCmd, args[1:], app)
	if errors.Cause(err) == ErrShowHelp {
		fmt.Println()
		fmt.Printf("%v: %v\n", aurora.Bold(aurora.White(aurora.Underline(strings.Join(fullCmd, " ")))), aurora.Cyan(cmd.Help()))
		fmt.Println()
		return nil
	}
	return err
}

func (c REPLCommands) Help() string {
	var longestCommandLength int
	for s := range c {
		if len(s) > longestCommandLength {
			longestCommandLength = len(s)
		}
	}
	var txt string
	for s, cmd := range c {
		difference := longestCommandLength - len(s)
		spaceAfter := strings.Repeat(" ", difference+4)
		txt += fmt.Sprintf("\n    %v%v %v", s, spaceAfter, aurora.Cyan(cmd.HelpText))
	}
	return txt
}

// func (c REPLCommands) Help() string {
// 	var longestCommandLength int
// 	for cmd := range c {
// 		if len(cmd) > longestCommandLength {
// 			longestCommandLength = len(cmd)
// 		}
// 	}

// 	var txts []string
// 	for s, cmd := range c {
// 		difference := longestCommandLength - len(s)
// 		spaceAfter := strings.Repeat(" ", difference+4)
// 		txts = append(txts, fmt.Sprintf("%v%v%v - %v\n", strings.Repeat(" ", indent*4), s, spaceAfter, cmd.Help(indent+1)))
// 	}
// 	return strings.Join(txts, "\n") + "\n"
// }

var ErrShowHelp = errors.New("")

var defaultREPLCommands = REPLCommands{
	"identity": REPLCommand{
		HelpText: "view the node's identity",
		Subcommands: REPLCommands{
			"mnemonic": CmdMnemonic,
			"address":  CmdAddress,
		},
	},
	"peers": REPLCommand{
		HelpText: "add, remove, and list peers",
		Subcommands: REPLCommands{
			"list": CmdPeers,
			"add":  CmdAddPeer,
			"rm": REPLCommand{
				HelpText: "remove peers from the peer store",
				Subcommands: REPLCommands{
					"all":        CmdRemoveAllPeers,
					"unverified": CmdRemoveUnverifiedPeers,
					"failed":     CmdRemoveFailedPeers,
				},
			},
			"dumpstore": CmdPeerStoreDebugPrint,
		},
	},
	"hush": REPLCommand{
		HelpText: "interact with the hush protocol",
		Subcommands: REPLCommands{
			"dumpstore": CmdHushStoreDebugPrint,
			"send":      CmdHushSendIndividualMessage,
			"sendgroup": CmdHushSendGroupMessage,
		},
	},
	"tree": REPLCommand{
		HelpText: "interact with the tree protocol",
		Subcommands: REPLCommands{
			"get":       CmdGetState,
			"set":       CmdSetState,
			"uris":      CmdStateURIs,
			"txs":       CmdListTxs,
			"subscribe": CmdSubscribe,
			"dumpstore": CmdTreeStoreDebugPrint,
		},
	},
	"blob": REPLCommand{
		HelpText: "interact with the blob protocol",
		Subcommands: REPLCommands{
			"list": CmdBlobs,
			"set": REPLCommand{
				HelpText: "configure the blob protocol",
				Subcommands: REPLCommands{
					"maxfetchconns": CmdSetBlobMaxFetchConns,
				},
			},
		},
	},
	"libp2p": REPLCommand{
		HelpText: "interact with the libp2p transport",
		Subcommands: REPLCommands{
			"id": CmdLibp2pPeerID,
		},
	},
	"ps": CmdProcessTree,
}

var (
	CmdMnemonic = REPLCommand{
		HelpText: "show your identity's mnemonic",
		Handler: func(args []string, app *App) error {
			m, err := app.KeyStore.Mnemonic()
			if err != nil {
				return err
			}
			app.Debugf("mnemonic: %v", m)
			return nil
		},
	}

	CmdAddress = REPLCommand{
		HelpText: "show your address",
		Handler: func(args []string, app *App) error {
			identity, err := app.KeyStore.DefaultPublicIdentity()
			if err != nil {
				return err
			}
			app.Debugf("address: %v", identity.Address())
			return nil
		},
	}

	CmdLibp2pPeerID = REPLCommand{
		HelpText: "show your libp2p peer ID",
		Handler: func(args []string, app *App) error {
			if app.Libp2pTransport == nil {
				return errors.New("libp2p is disabled")
			}
			peerID := app.Libp2pTransport.Libp2pPeerID()
			app.Debugf("libp2p peer ID: %v", peerID)
			return nil
		},
	}

	CmdSubscribe = REPLCommand{
		HelpText: "subscribe to a state URI",
		Handler: func(args []string, app *App) error {
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
		HelpText: "list all known state URIs",
		Handler: func(args []string, app *App) error {
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
		HelpText: "print the current state tree for a state URI",
		Handler: func(args []string, app *App) error {
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
		HelpText: "set a keypath in a state tree",
		Handler: func(args []string, app *App) error {
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
		HelpText: "list the txs for a given state URI",
		Handler: func(args []string, app *App) error {
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
		HelpText: "list all blobs",
		Handler: func(args []string, app *App) error {
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
		HelpText: "list all known peers",
		Handler: func(args []string, app *App) error {
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
		HelpText: "add a peer",
		Handler: func(args []string, app *App) error {
			if len(args) < 2 {
				return errors.New("requires 2 arguments: addpeer <transport> <dial addr>")
			}
			app.PeerStore.AddDialInfo(swarm.PeerDialInfo{args[0], args[1]}, "")
			return nil
		},
	}

	CmdRemoveAllPeers = REPLCommand{
		HelpText: "remove all peers",
		Handler: func(args []string, app *App) error {
			var toDelete []string
			for _, peer := range app.PeerStore.Peers() {
				toDelete = append(toDelete, peer.DeviceUniqueID())
			}
			app.PeerStore.RemovePeers(toDelete)
			return nil
		},
	}

	CmdRemoveUnverifiedPeers = REPLCommand{
		HelpText: "remove peers who haven't been verified",
		Handler: func(args []string, app *App) error {
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
		HelpText: "remove peers with more than a certain number of failures",
		Handler: func(args []string, app *App) error {
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
		HelpText: "send a 1:1 Hush message",
		Handler: func(args []string, app *App) error {
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
		HelpText: "send a group Hush message",
		Handler: func(args []string, app *App) error {
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
		HelpText: "print the contents of the protohush store",
		Handler: func(args []string, app *App) error {
			app.HushProtoStore.DebugPrint()
			return nil
		},
	}

	CmdTreeStoreDebugPrint = REPLCommand{
		HelpText: "print the contents of the prototree store",
		Handler: func(args []string, app *App) error {
			app.TreeProtoStore.DebugPrint()
			return nil
		},
	}

	CmdPeerStoreDebugPrint = REPLCommand{
		HelpText: "print the contents of the peer store",
		Handler: func(args []string, app *App) error {
			app.PeerStore.DebugPrint()
			return nil
		},
	}

	CmdProcessTree = REPLCommand{
		HelpText: "display the current process tree",
		Handler: func(args []string, app *App) error {
			app.Infof(0, "processes:\n%v", utils.PrettyJSON(app.ProcessTree()))
			return nil
		},
	}

	CmdSetBlobMaxFetchConns = REPLCommand{
		HelpText: "display the current process tree",
		Handler: func(args []string, app *App) error {
			app.Infof(0, "processes:\n%v", utils.PrettyJSON(app.ProcessTree()))
			return nil
		},
	}
)
