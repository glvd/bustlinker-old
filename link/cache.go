package link

import (
	"encoding/json"
	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/glvd/accipfs/config"
	"github.com/glvd/accipfs/core"
	"os"
	"path/filepath"
)

const (
	cacheDir     = ".cache"
	hashNodeName = "hashNodes"
	nodeName     = "nodes"
)

type baseCache struct {
	db           *badger.DB
	iteratorOpts badger.IteratorOptions
	cfg          *config.Config
}

type hashCache struct {
	baseCache
}

type nodeCache struct {
	baseCache
}

// Cacher ...
type Cacher interface {
	Load(hash string, data core.Unmarshaler) error
	Store(hash string, data core.Marshaler) error
	Update(hash string, fn func(bytes []byte) (core.Marshaler, error)) error
	Close() error
	Range(f func(hash string, value string) bool)
}

// DataHashInfo ...
type DataHashInfo struct {
	DataHash string            `json:"data_hash"`
	DataInfo core.Serializable `json:"data_info"`
	AddrInfo core.AddrInfo     `json:"addr_info"`
}

func newDataHashInfo(data core.Serializable) *DataHashInfo {
	return &DataHashInfo{
		DataHash: data.Hash(),
		DataInfo: data,
	}
}

// Hash ...
func (v DataHashInfo) Hash() string {
	return v.DataHash
}

// Marshal ...
func (v DataHashInfo) Marshal() ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal ...
func (v *DataHashInfo) Unmarshal(b []byte) error {
	return json.Unmarshal(b, v)
}

// HashCacher ...
func HashCacher(cfg *config.Config) Cacher {
	path := filepath.Join(cfg.Path, cacheDir, hashNodeName)
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			panic(err)
		}
	}
	opts := badger.DefaultOptions(path)
	opts.CompactL0OnClose = false
	opts.Truncate = true
	opts.ValueLogLoadingMode = options.FileIO
	opts.TableLoadingMode = options.MemoryMap
	opts.MaxTableSize = 16 << 20
	db, err := badger.Open(opts)
	if err != nil {
		panic(err)
	}
	itOpts := badger.DefaultIteratorOptions
	itOpts.Reverse = true
	return &hashCache{
		baseCache: baseCache{
			cfg:          cfg,
			iteratorOpts: itOpts,
			db:           db,
		},
	}
}

// Update ...
func (c *baseCache) Update(hash string, fn func(bytes []byte) (core.Marshaler, error)) error {
	return c.db.Update(
		func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(hash))
			if err != nil {
				return err
			}
			if fn != nil {
				return item.Value(func(val []byte) error {
					if fn != nil {
						data, err := fn(val)
						if err != nil {
							//do nothing when have err
							return err
						}
						encode, err := data.Marshal()
						if err != nil {
							return err
						}
						return txn.Set([]byte(hash), encode)
					}
					return nil
				})
			}
			return nil
		})
}

// SaveNode ...
func (c *baseCache) Store(hash string, data core.Marshaler) error {
	return c.db.Update(
		func(txn *badger.Txn) error {
			encode, err := data.Marshal()
			if err != nil {
				return err
			}
			return txn.Set([]byte(hash), encode)
		})
}

// LoadNode ...
func (c *baseCache) Load(hash string, data core.Unmarshaler) error {
	return c.db.View(
		func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(hash))
			if err != nil {
				return err
			}
			return item.Value(func(val []byte) error {
				return data.Unmarshal(val)
			})
		})
}

// Range ...
func (c *baseCache) Range(f func(key, value string) bool) {
	err := c.db.View(func(txn *badger.Txn) error {
		iter := txn.NewIterator(c.iteratorOpts)
		defer iter.Close()
		var item *badger.Item
		continueFlag := true
		for iter.Rewind(); iter.Valid(); iter.Next() {
			if !continueFlag {
				return nil
			}
			item = iter.Item()
			err := item.Value(func(v []byte) error {
				key := item.Key()
				val, err := item.ValueCopy(v)
				if err != nil {
					return err
				}
				continueFlag = f(string(key), string(val))
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Errorw("range data failed", "err", err)
	}
}

// Close ...
func (c *baseCache) Close() error {
	if c.db != nil {
		defer func() {
			c.db = nil
		}()
		return c.db.Close()
	}
	return nil
}

// NodeCacher ...
func NodeCacher(cfg *config.Config) Cacher {
	path := filepath.Join(cfg.Path, cacheDir, nodeName)
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			panic(err)
		}
	}
	opts := badger.DefaultOptions(path)
	opts.CompactL0OnClose = false
	opts.Truncate = true
	opts.ValueLogLoadingMode = options.FileIO
	opts.TableLoadingMode = options.MemoryMap
	//opts.ValueLogFileSize = 1<<28 - 1
	opts.MaxTableSize = 16 << 20
	db, err := badger.Open(opts)
	if err != nil {
		panic(err)
	}
	itOpts := badger.DefaultIteratorOptions
	itOpts.Reverse = true
	return &nodeCache{
		baseCache: baseCache{
			cfg:          cfg,
			iteratorOpts: itOpts,
			db:           db,
		},
	}
}
