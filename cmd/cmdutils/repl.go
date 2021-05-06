package cmdutils

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brynbellomy/klog"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"

	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type REPL struct {
	host     swarm.Host
	logger   log.Logger
	commands map[string]REPLCommand
	chDone   chan struct{}
	stopOnce sync.Once
}

type REPLCommand struct {
	HelpText string
	Handler  func(ctx context.Context, args []string, host swarm.Host, logger log.Logger) error
}

func NewREPL(host swarm.Host, logger log.Logger, commands map[string]REPLCommand) *REPL {
	return &REPL{host, logger, commands, make(chan struct{}), sync.Once{}}
}

func (r *REPL) Start() {
	flagset := flag.NewFlagSet("", flag.ContinueOnError)
	klog.InitFlags(flagset)
	flagset.Set("logtostderr", "true")
	flagset.Set("v", "2")
	klog.SetFormatter(&klog.FmtConstWidth{
		FileNameCharWidth: 24,
		UseColor:          true,
	})
	klog.Flush()
	defer klog.Flush()

	fmt.Println("Type \"help\" for a list of commands.")
	fmt.Println()

	var longestCommandLength int
	for cmd := range r.commands {
		if len(cmd) > longestCommandLength {
			longestCommandLength = len(cmd)
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("> ")

		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		parts := strings.Split(line, " ")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}

		if len(parts) < 1 {
			r.logger.Error("enter a command")
			continue
		} else if parts[0] == "help" {
			fmt.Println("___ Commands _________")
			fmt.Println()
			for cmd, info := range r.commands {
				difference := longestCommandLength - len(cmd)
				space := strings.Repeat(" ", difference+4)
				fmt.Printf("%v%v- %v\n", cmd, space, info.HelpText)
			}
			continue
		}

		cmd, exists := r.commands[parts[0]]
		if !exists {
			r.logger.Error("unknown command")
			continue
		}

		func() {
			ctx, cancel := utils.ContextFromChan(r.chDone)
			defer cancel()

			err := cmd.Handler(ctx, parts[1:], r.host, r.logger)
			if err != nil {
				r.logger.Error(err)
			}
		}()
	}
}

func (r *REPL) Stop() {
	r.stopOnce.Do(func() {
		close(r.chDone)
	})
}

func (r *REPL) Done() <-chan struct{} {
	return r.chDone
}

var DefaultREPLCommands = map[string]REPLCommand{
	"libp2pid": {
		"list your libp2p peer ID",
		func(ctx context.Context, args []string, host swarm.Host, logger log.Logger) error {
			type peerIDer interface {
				Libp2pPeerID() string
			}
			peerID := host.Transport("libp2p").(peerIDer).Libp2pPeerID()
			logger.Debugf("libp2p peer ID: %v", peerID)
			return nil
		},
	},
	"subscribe": {
		"subscribe",
		func(ctx context.Context, args []string, host swarm.Host, logger log.Logger) error {
			if len(args) < 1 {
				return errors.New("missing argument: state URI")
			}
			stateURI := args[0]
			sub, err := host.Subscribe(ctx, stateURI, swarm.SubscriptionType_Txs, nil, &swarm.FetchHistoryOpts{FromTxID: tree.GenesisTxID})
			if err != nil {
				return err
			}
			defer sub.Close()
			return nil
		},
	},
	"stateuris": {
		"list all known state URIs",
		func(ctx context.Context, args []string, host swarm.Host, logger log.Logger) error {
			stateURIs, err := host.Controllers().KnownStateURIs()
			if err != nil {
				return err
			}
			if len(stateURIs) == 0 {
				logger.Info(0, "no known state URIs")
			} else {
				for _, stateURI := range stateURIs {
					fmt.Println("- ", stateURI)
				}
			}
			return nil
		},
	},
	"state": {
		"print the current state tree",
		func(ctx context.Context, args []string, host swarm.Host, logger log.Logger) error {
			if len(args) < 1 {
				return errors.New("missing argument: state URI")
			}

			stateURI := args[0]
			node, err := host.Controllers().StateAtVersion(stateURI, nil)
			if err != nil {
				return err
			}
			defer node.Close()
			var keypath state.Keypath
			var rng *state.Range
			if len(args) > 1 {
				_, keypath, rng, err = tree.ParsePatchPath([]byte(args[1]))
				if err != nil {
					return err
				}
			}
			logger.Debugf("stateURI: %v / keypath: %v / range: %v", stateURI, keypath, rng)
			node = node.NodeAt(keypath, rng)
			node.DebugPrint(logger.Debugf, false, 0)
			logger.Debug(utils.PrettyJSON(node))
			return nil
		},
	},
	"blobs": {
		"list all blobs",
		func(ctx context.Context, args []string, host swarm.Host, logger log.Logger) error {
			blobIDsNeeded, err := host.BlobStore().RefsNeeded()
			if err != nil {
				return err
			}

			blobIDs, err := host.BlobStore().AllHashes()
			if err != nil {
				return err
			}

			var rows [][]string

			for _, id := range blobIDsNeeded {
				rows = append(rows, []string{id.String(), "(missing)"})
			}

			for _, id := range blobIDs {
				rows = append(rows, []string{id.String(), ""})
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
			table.SetCenterSeparator("|")
			table.SetRowLine(true)
			table.SetHeader([]string{"ID", "Status"})
			table.AppendBulk(rows)
			table.Render()

			return nil
		},
	},
	"peers": {
		"list all known peers",
		func(ctx context.Context, args []string, host swarm.Host, logger log.Logger) error {
			fmtPeerRow := func(addr, dialAddr string, lastContact, lastFailure time.Time, failures uint64, remainingBackoff time.Duration, stateURIs []string) []string {
				if len(addr) > 10 {
					addr = addr[:4] + "..." + addr[len(addr)-4:]
				}
				if len(dialAddr) > 30 {
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
				return []string{addr, dialAddr, lastContactStr, lastFailureStr, failuresStr, remainingBackoffStr, fmt.Sprintf("%v", stateURIs)}
			}

			var data [][]string
			for _, peer := range host.Peers() {
				for _, addr := range peer.Addresses() {
					data = append(data, fmtPeerRow(addr.Hex(), peer.DialInfo().DialAddr, peer.LastContact(), peer.LastFailure(), peer.Failures(), peer.RemainingBackoff(), peer.StateURIs().Slice()))
				}
				if len(peer.Addresses()) == 0 {
					data = append(data, fmtPeerRow("?", peer.DialInfo().DialAddr, peer.LastContact(), peer.LastFailure(), peer.Failures(), peer.RemainingBackoff(), peer.StateURIs().Slice()))
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
			table.SetHeader([]string{"Address", "DialAddr", "LastContact", "LastFailure", "Failures", "Backoff", "StateURIs"})
			table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
			table.SetColumnColor(
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
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
	},
	"addpeer": {
		"add a peer",
		func(ctx context.Context, args []string, host swarm.Host, logger log.Logger) error {
			if len(args) < 2 {
				return errors.New("requires 2 arguments: addpeer <transport> <dial addr>")
			}
			host.AddPeer(swarm.PeerDialInfo{args[0], args[1]})
			return nil
		},
	},
	"rmallpeers": {
		"remove all peers",
		func(ctx context.Context, args []string, host swarm.Host, logger log.Logger) error {
			var toDelete []swarm.PeerDialInfo
			for _, peer := range host.Peers() {
				toDelete = append(toDelete, peer.DialInfo())
			}
			host.RemovePeers(toDelete)
			return nil
		},
	},
	"rmunverifiedpeers": {
		"remove peers who haven't been verified",
		func(ctx context.Context, args []string, host swarm.Host, logger log.Logger) error {
			var toDelete []swarm.PeerDialInfo
			for _, peer := range host.Peers() {
				if len(peer.Addresses()) == 0 {
					toDelete = append(toDelete, peer.DialInfo())
				}
			}
			host.RemovePeers(toDelete)
			return nil
		},
	},
	"rmfailedpeers": {
		"remove peers with more than a certain number of failures",
		func(ctx context.Context, args []string, host swarm.Host, logger log.Logger) error {
			if len(args) < 1 {
				return errors.New("requires 1 argument: rmfailedpeers <number of failures>")
			}

			num, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrap(err, "bad argument")
			}

			var toDelete []swarm.PeerDialInfo
			for _, peer := range host.Peers() {
				if peer.Failures() > uint64(num) {
					toDelete = append(toDelete, peer.DialInfo())
				}
			}
			host.RemovePeers(toDelete)
			return nil
		},
	},
	"set": {
		"set a keypath in a state tree",
		func(ctx context.Context, args []string, host swarm.Host, logger log.Logger) error {
			if len(args) < 3 {
				return errors.New("requires 3 arguments: set <state URI> <keypath> <JSON value>")
			}
			stateURI := args[0]
			keypath := state.Keypath(args[1])
			jsonVal := strings.Join(args[2:], " ")
			var val interface{}
			err := json.Unmarshal([]byte(jsonVal), &val)
			if err != nil {
				return err
			}
			err = host.SendTx(context.TODO(), tree.Tx{
				ID:       types.RandomID(),
				StateURI: stateURI,
				Patches: []tree.Patch{{
					Keypath: keypath,
					Val:     val,
				}},
			})
			return nil
		},
	},
}
