package link

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"sync"
)

type Address struct {
	lock      *sync.RWMutex
	addresses map[peer.ID]peer.AddrInfo
}

func NewAddress() *Address {
	return &Address{
		lock:      &sync.RWMutex{},
		addresses: make(map[peer.ID]peer.AddrInfo),
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
