package link

import (
	"github.com/glvd/bustlinker/config"
	"github.com/libp2p/go-libp2p-core/peer"
	"sync"
)

type Address struct {
	lock      *sync.RWMutex
	addresses map[peer.ID]peer.AddrInfo
	cache     Cacher
}

func NewAddress(config config.CacheConfig) *Address {
	return &Address{
		lock:      &sync.RWMutex{},
		addresses: make(map[peer.ID]peer.AddrInfo),
		cache:     HashCacher(config),
	}
}

func (a *Address) CheckPeerAddress(id peer.ID) (b bool) {
	a.lock.RLock()
	_, b = a.addresses[id]
	a.lock.RUnlock()
	return
}

func (a *Address) AddPeerAddress(id peer.ID, addrs peer.AddrInfo) (b bool) {
	a.lock.RLock()
	_, b = a.addresses[id]
	a.lock.RUnlock()
	if b {
		return !b
	}
	a.lock.Lock()
	_, b = a.addresses[id]
	if !b {
		a.addresses[id] = addrs
	}
	a.lock.Unlock()
	return !b
}

func (a *Address) GetAddress(id peer.ID) (ai peer.AddrInfo, b bool) {
	a.lock.RLock()
	ai, b = a.addresses[id]
	a.lock.RUnlock()
	return ai, b
}

func (a *Address) Peers() (ids []peer.ID) {
	a.lock.RLock()
	for id := range a.addresses {
		ids = append(ids, id)
	}
	a.lock.RUnlock()
	return
}
