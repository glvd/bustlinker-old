package link

import (
	"fmt"
	"github.com/glvd/bustlinker/core"
	"github.com/libp2p/go-libp2p-core/peer"
	"sync"
)

type Hash struct {
	lock   *sync.RWMutex
	hashes map[string]map[peer.ID]bool
	cache  Cacher
}

func defaultHash() *Hash {
	return &Hash{
		lock:   &sync.RWMutex{},
		hashes: make(map[string]map[peer.ID]bool),
	}
}

func NewHash(node *core.IpfsNode) *Hash {
	hash := defaultHash()
	cfg, err := node.Repo.LinkConfig()
	if err != nil {
		return hash
	}
	fmt.Println("cache initialized")
	hash.cache = NewCache(cfg.Hash, node.Repo.Path(), hashName)
	return hash
}

func (h *Hash) Get(hash string) peer.IDSlice {
	var ids map[peer.ID]bool
	h.lock.RLock()
	ids = h.hashes[hash]
	h.lock.RUnlock()
	var peers peer.IDSlice
	for id := range ids {
		peers = append(peers, id)
	}
	return peers
}

func (h *Hash) CheckHash(hash string) (b bool) {
	h.lock.RLock()
	_, b = h.hashes[hash]
	h.lock.RUnlock()
	return b
}

func (h *Hash) CheckHashPeer(hash string, id peer.ID) (b bool) {
	var ids map[peer.ID]bool
	h.lock.RLock()
	ids, b = h.hashes[hash]
	if b {
		_, b = ids[id]
	}
	h.lock.RUnlock()
	return b
}

func (h *Hash) Add(hash string, id peer.ID) (b bool) {
	if h.CheckHashPeer(hash, id) {
		return false
	}
	var ids map[peer.ID]bool
	h.lock.Lock()
	ids, b = h.hashes[hash]
	if b {
		_, b = ids[id]
		ids[id] = true
	} else {
		b = true
		h.hashes[hash] = map[peer.ID]bool{
			id: true,
		}
	}
	h.lock.Unlock()
	return b
}
