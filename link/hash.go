package link

import (
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
