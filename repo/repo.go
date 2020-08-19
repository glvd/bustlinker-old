package repo

import (
	"errors"
	"github.com/glvd/bustlinker/config"
	ipfsconfig "github.com/ipfs/go-ipfs-config"
	"io"

	keystore "github.com/glvd/bustlinker/keystore"
	filestore "github.com/ipfs/go-filestore"

	ds "github.com/ipfs/go-datastore"
	ma "github.com/multiformats/go-multiaddr"
)

var (
	ErrApiNotRunning = errors.New("api not running")
)

// Repo represents all persistent data of a given ipfs node.
type Repo interface {
	// IPFSConfig returns the ipfs configuration file from the repo. Changes made
	// to the returned ipfsconfig are not automatically persisted.
	IPFSConfig() (*ipfsconfig.Config, error)

	LinkConfig() (*config.LinkConfig, error)

	Config() (*config.Config, error)

	// BackupConfig creates a backup of the current configuration file using
	// the given prefix for naming.
	BackupConfig(prefix string) (string, error)

	// SetIPFSConfig persists the given configuration struct to storage.
	SetIPFSConfig(*ipfsconfig.Config) error
	SetLinkConfig(*config.LinkConfig) error
	SetConfig(*config.Config) error

	// SetConfigKey sets the given key-value pair within the ipfsconfig and persists it to storage.
	SetConfigKey(key string, value interface{}) error

	// GetConfigKey reads the value for the given key from the configuration in storage.
	GetConfigKey(key string) (interface{}, error)

	// Datastore returns a reference to the configured data storage backend.
	Datastore() Datastore

	// GetStorageUsage returns the number of bytes stored.
	GetStorageUsage() (uint64, error)

	// Keystore returns a reference to the key management interface.
	Keystore() keystore.Keystore

	// FileManager returns a reference to the filestore file manager.
	FileManager() *filestore.FileManager

	// SetAPIAddr sets the API address in the repo.
	SetAPIAddr(addr ma.Multiaddr) error

	// SwarmKey returns the configured shared symmetric key for the private networks feature.
	SwarmKey() ([]byte, error)

	io.Closer
}

// Datastore is the interface required from a datastore to be
// acceptable to FSRepo.
type Datastore interface {
	ds.Batching // must be thread-safe
}
