package redwood

import (
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Node          *NodeConfig          `yaml:"Node"`
	P2PTransport  *P2PTransportConfig  `yaml:"P2PTransport"`
	HTTPTransport *HTTPTransportConfig `yaml:"HTTPTransport"`
	HTTPRPC       *HTTPRPCConfig       `yaml:"HTTPRPC"`

	configPath string       `yaml:"-"`
	mu         sync.RWMutex `yaml:"-"`
}

type NodeConfig struct {
	HDMnemonicPhrase        string          `yaml:"HDMnemonicPhrase"`
	BootstrapPeers          []BootstrapPeer `yaml:"BootstrapPeers"`
	SubscribedStateURIs     []string        `yaml:"SubscribedStateURIs"`
	MaxPeersPerSubscription uint64          `yaml:"MaxPeersPerSubscription"`
	DataRoot                string          `yaml:"DataRoot"`
	DevMode                 bool            `yaml:"DevMode"`
}

type BootstrapPeer struct {
	Transport     string   `yaml:"Transport"`
	DialAddresses []string `yaml:"DialAddresses"`
}

type P2PTransportConfig struct {
	Enabled    bool   `yaml:"Enabled"`
	KeyFile    string `yaml:"KeyFile"`
	ListenAddr string `yaml:"ListenAddr"`
	ListenPort uint   `yaml:"ListenPort"`
}

type HTTPTransportConfig struct {
	Enabled         bool   `yaml:"Enabled"`
	ListenHost      string `yaml:"ListenHost"`
	CookieSecret    string `yaml:"CookieSecret"`
	DefaultStateURI string `yaml:"DefaultStateURI"`
}

type HTTPRPCConfig struct {
	Enabled    bool   `yaml:"Enabled"`
	ListenHost string `yaml:"ListenHost"`
}

var DefaultConfig = func() Config {
	configRoot, err := ConfigRoot()
	if err != nil {
		panic(err)
	}
	err = os.MkdirAll(configRoot, 0700)
	if err != nil {
		panic(err)
	}

	dataRoot, err := dataRoot()
	if err != nil {
		panic(err)
	}

	hdMnemonicPhrase, err := GenerateMnemonic()
	if err != nil {
		panic(err)
	}

	httpCookieSecret := make([]byte, 32)
	_, err = rand.Read(httpCookieSecret)
	if err != nil {
		panic(err)
	}

	return Config{
		Node: &NodeConfig{
			HDMnemonicPhrase:        hdMnemonicPhrase,
			BootstrapPeers:          []BootstrapPeer{},
			SubscribedStateURIs:     []string{},
			MaxPeersPerSubscription: 4,
			DataRoot:                dataRoot,
			DevMode:                 false,
		},
		P2PTransport: &P2PTransportConfig{
			Enabled:    true,
			KeyFile:    filepath.Join(configRoot, "p2p_key"),
			ListenAddr: "0.0.0.0",
			ListenPort: 21231,
		},
		HTTPTransport: &HTTPTransportConfig{
			Enabled:         true,
			ListenHost:      ":8080",
			CookieSecret:    string(httpCookieSecret),
			DefaultStateURI: "",
		},
		HTTPRPC: &HTTPRPCConfig{
			Enabled:    false,
			ListenHost: ":8081",
		},
	}
}()

func ConfigRoot() (root string, _ error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		configRoot, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	configRoot = filepath.Join(configRoot, "redwood")
	return configRoot, nil
}

func dataRoot() (string, error) {
	switch runtime.GOOS {
	case "windows", "darwin", "plan9":
		configRoot, err := ConfigRoot()
		if err != nil {
			return "", err
		}
		return configRoot, nil

	default: // unix/linux
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		return filepath.Join(homeDir, ".local", "share", "redwood"), nil
	}
}

func ReadConfigAtPath(configPath string) (*Config, error) {
	if configPath == "" {
		var err error
		configPath, err = ConfigRoot()
		if err != nil {
			return nil, err
		}
		configPath = filepath.Join(configPath, ".redwoodrc")
	}

	// Copy the default config
	cfg := DefaultConfig

	bs, err := ioutil.ReadFile(configPath)
	// If the file can't be found, we ignore the error.  Otherwise, return it.
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Decode the config file on top of the defaults
	err = yaml.Unmarshal(bs, &cfg)
	if err != nil {
		return nil, err
	}

	// Save the file again in case it didn't exist or was missing fields
	cfg.configPath = configPath
	err = cfg.save()
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Read(fn func()) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	fn()
}

func (c *Config) Update(fn func() error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := fn()
	if err != nil {
		return err
	}

	return c.save()
}

func (c *Config) save() error {
	f, err := os.OpenFile(c.configPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0700)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	encoder.SetIndent(4)

	err = encoder.Encode(c)
	if err != nil {
		return err
	}
	return nil
}

func (c *Config) Path() string {
	return c.configPath
}

func (c *Config) RefDataRoot() string {
	return filepath.Join(c.Node.DataRoot, "refs")
}

func (c *Config) TxDBRoot() string {
	return filepath.Join(c.Node.DataRoot, "txs")
}

func (c *Config) StateDBRoot() string {
	return filepath.Join(c.Node.DataRoot, "states")
}

type Duration time.Duration

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

func (d *Duration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}
