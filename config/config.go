package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	config "github.com/ipfs/go-ipfs-config"
)

type IPFSConfig = config.Config

type LinkConfig struct {
	Addresses []string
}

// Config ...
type Config struct {
	IPFS *IPFSConfig
	Link *LinkConfig
}

// Clone copies the config. Use when updating.
func (c *Config) Clone() (*Config, error) {
	var newConfig Config
	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(c); err != nil {
		return nil, fmt.Errorf("failure to encode config: %s", err)
	}

	if err := json.NewDecoder(&buf).Decode(&newConfig); err != nil {
		return nil, fmt.Errorf("failure to decode config: %s", err)
	}

	return &newConfig, nil
}

func FromMap(v map[string]interface{}) (*Config, error) {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(v); err != nil {
		return nil, err
	}
	var conf Config
	if err := json.NewDecoder(buf).Decode(&conf); err != nil {
		return nil, fmt.Errorf("failure to decode config: %s", err)
	}
	return &conf, nil
}

func ToMap(conf *Config) (map[string]interface{}, error) {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(conf); err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.NewDecoder(buf).Decode(&m); err != nil {
		return nil, fmt.Errorf("failure to decode config: %s", err)
	}
	return m, nil
}

func InitLinkConfig(repoPath string, conf *Config) (*LinkConfig, error) {
	var cfg LinkConfig
	cfg.Addresses = defaultAddresses()
	return &cfg, nil
}

func defaultAddresses() []string {
	return []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/tcp/4001"}
}
