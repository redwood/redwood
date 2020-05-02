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
	*NodeConfig      `yaml:"Node"`
	*RPCClientConfig `yaml:"RPCClient"`

	configPath string     `yaml:"-"`
	mu         sync.Mutex `yaml:"-"`
}

type NodeConfig struct {
	P2PKeyFile              string   `yaml:"P2PKeyFile"`
	P2PListenAddr           string   `yaml:"P2PListenAddr"`
	P2PListenPort           uint     `yaml:"P2PListenPort"`
	BootstrapPeers          []string `yaml:"BootstrapPeers"`
	RPCListenNetwork        string   `yaml:"RPCListenNetwork"`
	RPCListenHost           string   `yaml:"RPCListenHost"`
	HTTPListenHost          string   `yaml:"HTTPListenHost"`
	HTTPCookieSecret        string   `yaml:"HTTPCookieSecret"`
	DevMode                 bool     `yaml:"DevMode"`
	HDMnemonicPhrase        string   `yaml:"HDMnemonicPhrase"`
	ContentAnnounceInterval Duration `yaml:"ContentAnnounceInterval"`
	ContentRequestInterval  Duration `yaml:"ContentRequestInterval"`
	FindProviderTimeout     Duration `yaml:"FindProviderTimeout"`
	DefaultStateURI         string   `yaml:"DefaultStateURI"`
	StateURIs               []string `yaml:"StateURIs"`
	DataRoot                string   `yaml:"DataRoot"`
}

type RPCClientConfig struct {
	Host string `yaml:"Host"`
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
		NodeConfig: &NodeConfig{
			P2PKeyFile:              filepath.Join(configRoot, "p2p_key"),
			P2PListenAddr:           "0.0.0.0",
			P2PListenPort:           21231,
			RPCListenNetwork:        "tcp",
			RPCListenHost:           "0.0.0.0:21232",
			HTTPListenHost:          ":8080",
			HTTPCookieSecret:        string(httpCookieSecret),
			HDMnemonicPhrase:        hdMnemonicPhrase,
			ContentAnnounceInterval: Duration(15 * time.Second),
			ContentRequestInterval:  Duration(15 * time.Second),
			FindProviderTimeout:     Duration(10 * time.Second),
			StateURIs:               []string{},
			DataRoot:                dataRoot,
			BootstrapPeers: []string{
				"/dns4/jupiter.axon.science/tcp/1337/p2p/16Uiu2HAm4cL1W1yHcsQuDp9R19qeyAewekCdqyVM39WMykjVL2mt",
				"/dns4/saturn.axon.science/tcp/1337/p2p/16Uiu2HAkvBf1UUPvSFFyGWd5bECPc58qrMbiis2JW8q1AZG8zUgH",
			},
		},
		RPCClientConfig: &RPCClientConfig{
			Host: "0.0.0.0:21232",
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
	return filepath.Join(c.DataRoot, "refs")
}

func (c *Config) TxDBRoot() string {
	return filepath.Join(c.DataRoot, "txs")
}

func (c *Config) StateDBRoot() string {
	return filepath.Join(c.DataRoot, "states")
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
