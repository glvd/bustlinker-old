package link

import (
	"encoding/json"
	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/glvd/accipfs/core"
	"github.com/glvd/bustlinker/config"
	"os"
	"path/filepath"
)

const (
	cacheDir    = ".cache"
	hashName    = "hash"
	addressName = "address"
	nodeName    = "node"
)

type baseCache struct {
	db           *badger.DB
	iteratorOpts badger.IteratorOptions
	cfg          config.CacheConfig
}

type hashCache struct {
	baseCache
}

type nodeCache struct {
	baseCache
}

// Cacher ...
type Cacher interface {
	Load(hash string, data json.Unmarshaler) error
	Store(hash string, data json.Marshaler) error
	Update(hash string, up CacheUpdater) error
	Close() error
	Range(f func(hash string, value string) bool)
}

type CacheUpdater interface {
	json.Unmarshaler
	Do()
	json.Marshaler
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

// NewCache ...
func NewCache(cfg config.CacheConfig, path, name string) Cacher {
	path = filepath.Join(path, name)
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

func (c *baseCache) UpdateBytes(hash string, b []byte) error {
	return c.db.Update(
		func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(hash))
			if err != nil {
				return err
			}
			return item.Value(func(val []byte) error {
				return txn.Set([]byte(hash), b)
			})
			return nil
		})
}

// Update ...
func (c *baseCache) Update(hash string, up CacheUpdater) error {
	return c.db.Update(
		func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(hash))
			if err != nil {
				return err
			}
			if up != nil {
				return item.Value(func(val []byte) error {
					err := up.UnmarshalJSON(val)
					if err != nil {
						//do nothing when have err
						return err
					}
					up.Do()
					encode, err := up.MarshalJSON()
					if err != nil {
						return err
					}
					return txn.Set([]byte(hash), encode)
					return nil
				})
			}
			return nil
		})
}

// SaveNode ...
func (c *baseCache) Store(hash string, data json.Marshaler) error {
	return c.db.Update(
		func(txn *badger.Txn) error {
			encode, err := data.MarshalJSON()
			if err != nil {
				return err
			}
			return txn.Set([]byte(hash), encode)
		})
}

// LoadNode ...
func (c *baseCache) Load(hash string, data json.Unmarshaler) error {
	return c.db.View(
		func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(hash))
			if err != nil {
				return err
			}
			return item.Value(func(val []byte) error {
				return data.UnmarshalJSON(val)
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
			err := iter.Item().Value(func(v []byte) error {
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
