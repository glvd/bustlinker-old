package link

import (
	"github.com/glvd/bustlinker/core"
	"github.com/libp2p/go-libp2p-core/peer"
	"sync"
)

const hashName = "hash"

type HashCache struct {
	lock   *sync.RWMutex
	hashes map[string]map[peer.ID]bool
	cache  Cacher
}

func hashCache() *HashCache {
	return &HashCache{
		lock:   &sync.RWMutex{},
		hashes: make(map[string]map[peer.ID]bool),
	}
}

func NewHash(node *core.IpfsNode) *HashCache {
	cache := hashCache()
	cfg, err := node.Repo.LinkConfig()
	if err != nil {
		return cache
	}
	log.Info("hash cache initialized")
	cache.cache = NewCache(cfg.Hash, node.Repo.Path(), hashName)
	return cache
}

func (c *HashCache) Get(hash string) peer.IDSlice {
	var ids map[peer.ID]bool
	var peers peer.IDSlice
	c.lock.RLock()
	ids = c.hashes[hash]
	for id := range ids {
		peers = append(peers, id)
	}
	c.lock.RUnlock()

	return peers
}

func (c *HashCache) CheckHash(hash string) (b bool) {
	c.lock.RLock()
	_, b = c.hashes[hash]
	c.lock.RUnlock()
	return b
}

func (c *HashCache) CheckHashPeer(hash string, id peer.ID) (b bool) {
	var ids map[peer.ID]bool
	c.lock.RLock()
	ids, b = c.hashes[hash]
	if b {
		_, b = ids[id]
	}
	c.lock.RUnlock()
	return b
}

func (c *HashCache) Add(hash string, id peer.ID) (b bool) {
	if c.CheckHashPeer(hash, id) {
		return false
	}
	var ids map[peer.ID]bool
	c.lock.Lock()
	ids, b = c.hashes[hash]
	if b {
		_, b = ids[id]
		ids[id] = true
	} else {
		b = false
		c.hashes[hash] = map[peer.ID]bool{
			id: true,
		}
	}
	c.lock.Unlock()
	return !b
}
