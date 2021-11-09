package cmdutils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"

	"redwood.dev/errors"
	"redwood.dev/rpc"
	"redwood.dev/utils"
)

type Config struct {
	Mode       Mode       `yaml:"-"`
	REPLConfig REPLConfig `yaml:"-"`

	BootstrapPeers  []BootstrapPeer `yaml:"BootstrapPeers"`
	DataRoot        string          `yaml:"DataRoot"`
	DNSOverHTTPSURL string          `yaml:"DNSOverHTTPSURL"`
	JWTSecret       string          `yaml:"JWTSecret"`
	DevMode         bool            `yaml:"-"`

	KeyStore KeyStoreConfig `yaml:"-"`

	Libp2pTransport    Libp2pTransportConfig    `yaml:"Libp2pTransport"`
	BraidHTTPTransport BraidHTTPTransportConfig `yaml:"BraidHTTPTransport"`

	AuthProtocol AuthProtocolConfig `yaml:"AuthProtocol"`
	BlobProtocol BlobProtocolConfig `yaml:"BlobProtocol"`
	HushProtocol HushProtocolConfig `yaml:"HushProtocol"`
	TreeProtocol TreeProtocolConfig `yaml:"TreeProtocol"`

	HTTPRPC *rpc.HTTPConfig `yaml:"HTTPRPC"`

	configPath string `yaml:"-"`
}

type REPLConfig struct {
	Prompt   string
	Commands []REPLCommand
}

type BootstrapPeer struct {
	Transport     string   `yaml:"Transport"`
	DialAddresses []string `yaml:"DialAddresses"`
}

type KeyStoreConfig struct {
	Password             string `yaml:"-"`
	Mnemonic             string `yaml:"-"`
	InsecureScryptParams bool   `yaml:"-"`
}

type Libp2pTransportConfig struct {
	Enabled      bool     `yaml:"Enabled"`
	ListenAddr   string   `yaml:"ListenAddr"`
	ListenPort   uint     `yaml:"ListenPort"`
	Reachability string   `yaml:"Reachability"`
	ReachableAt  string   `yaml:"ReachableAt"`
	StaticRelays []string `yaml:"StaticRelays"`
}

type BraidHTTPTransportConfig struct {
	Enabled         bool   `yaml:"Enabled"`
	ListenHost      string `yaml:"ListenHost"`
	ListenHostSSL   string `yaml:"ListenHostSSL"`
	TLSCertFile     string `yaml:"TLSCertFile"`
	TLSKeyFile      string `yaml:"TLSKeyFile"`
	DefaultStateURI string `yaml:"DefaultStateURI"`
	ReachableAt     string `yaml:"ReachableAt"`
}

type AuthProtocolConfig struct {
	Enabled bool `yaml:"Enabled"`
}

type BlobProtocolConfig struct {
	Enabled bool `yaml:"Enabled"`
}

type HushProtocolConfig struct {
	Enabled bool `yaml:"Enabled"`
}

type TreeProtocolConfig struct {
	Enabled                 bool   `yaml:"Enabled"`
	MaxPeersPerSubscription uint64 `yaml:"MaxPeersPerSubscription"`
}

func DefaultConfig(appName string) Config {
	configRoot, err := DefaultConfigRoot(appName)
	if err != nil {
		panic(err)
	}
	err = os.MkdirAll(configRoot, 0777|os.ModeDir)
	if err != nil {
		panic(err)
	}

	dataRoot, err := DefaultDataRoot(appName)
	if err != nil {
		panic(err)
	}

	return Config{
		Mode: ModeREPL,
		REPLConfig: REPLConfig{
			Prompt: ">",
			Commands: []REPLCommand{
				CmdMnemonic,
				CmdAddress,
				CmdLibp2pPeerID,
				CmdSubscribe,
				CmdStateURIs,
				CmdGetState,
				CmdSetState,
				CmdListTxs,
				CmdBlobs,
				CmdPeers,
				CmdAddPeer,
				CmdRemoveAllPeers,
				CmdRemoveUnverifiedPeers,
				CmdRemoveFailedPeers,
				CmdHushSendIndividualMessage,
				CmdHushSendGroupMessage,
				CmdHushStoreDebugPrint,
				CmdTreeStoreDebugPrint,
				CmdPeerStoreDebugPrint,
				CmdProcessTree,
			},
		},
		BootstrapPeers: []BootstrapPeer{},
		DataRoot:       dataRoot,
		JWTSecret:      utils.RandomString(32),
		DevMode:        false,
		Libp2pTransport: Libp2pTransportConfig{
			Enabled:    true,
			ListenAddr: "0.0.0.0",
			ListenPort: 21231,
		},
		BraidHTTPTransport: BraidHTTPTransportConfig{
			Enabled:         true,
			ListenHost:      ":8080",
			ListenHostSSL:   ":8082",
			DefaultStateURI: "",
		},
		AuthProtocol: AuthProtocolConfig{
			Enabled: true,
		},
		BlobProtocol: BlobProtocolConfig{
			Enabled: true,
		},
		HushProtocol: HushProtocolConfig{
			Enabled: true,
		},
		TreeProtocol: TreeProtocolConfig{
			Enabled:                 true,
			MaxPeersPerSubscription: 4,
		},
		HTTPRPC: &rpc.HTTPConfig{
			Enabled:    false,
			ListenHost: ":8081",
		},
	}
}

func DefaultConfigRoot(appName string) (root string, _ error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		configRoot, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	configRoot = filepath.Join(configRoot, appName)
	return configRoot, nil
}

func DefaultConfigPath(appName string) (root string, _ error) {
	configRoot, err := DefaultConfigRoot(appName)
	if err != nil {
		return "", err
	}
	return filepath.Join(configRoot, ".redwoodrc"), nil
}

func DefaultDataRoot(appName string) (string, error) {
	switch runtime.GOOS {
	case "windows", "darwin", "plan9":
		configRoot, err := DefaultConfigRoot(appName)
		if err != nil {
			return "", err
		}
		return configRoot, nil

	default: // unix/linux
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		return filepath.Join(homeDir, ".local", "share", appName), nil
	}
}

func FindOrCreateConfigAtPath(dst *Config, appName, configPath string) error {
	if dst == nil {
		*dst = Config{}
	}

	if configPath == "" {
		var err error
		configPath, err = DefaultConfigRoot(appName)
		if err != nil {
			return err
		}
		configPath = filepath.Join(configPath, ".redwoodrc")
	}

	bs, err := ioutil.ReadFile(configPath)
	// If the file can't be found, we ignore the error.  Otherwise, return it.
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Decode the config file on top of whatever was passed in
	err = yaml.Unmarshal(bs, dst)
	if err != nil {
		return err
	}

	// Save the file again in case it didn't exist or was missing fields
	dst.configPath = configPath
	err = dst.Save()
	if err != nil {
		return err
	}
	return nil
}

func (c *Config) Save() error {
	f, err := os.OpenFile(c.configPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0700)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	encoder.SetIndent(4)
	return encoder.Encode(c)
}

func (c *Config) BlobDataRoot() string {
	return filepath.Join(c.DataRoot, "blobs")
}

func (c *Config) TxDBRoot() string {
	return filepath.Join(c.DataRoot, "txs")
}

func (c *Config) StateDBRoot() string {
	return filepath.Join(c.DataRoot, "states")
}

func (c *Config) KeyStoreRoot() string {
	return filepath.Join(c.DataRoot, "keystore")
}

type Mode int

const (
	ModeREPL Mode = iota
	ModeTermUI
	ModeHeadless
)

func (m *Mode) UnmarshalText(bs []byte) error {
	switch string(bs) {
	case "repl":
		*m = ModeREPL
	case "termui":
		*m = ModeTermUI
	case "headless":
		*m = ModeHeadless
	default:
		return errors.Errorf("unknown mode '%v'", string(bs))
	}
	return nil
}
