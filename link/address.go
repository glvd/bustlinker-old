package link

import (
	"context"
	"fmt"
	"github.com/glvd/bustlinker/core"
	"github.com/libp2p/go-libp2p-core/peer"
	"sync"
)

type RootPath interface {
	Path() string
}

type Address struct {
	lock      *sync.RWMutex
	addresses map[peer.ID]peer.AddrInfo
	cache     Cacher
}

func defaultAddress() *Address {
	return &Address{
		lock:      &sync.RWMutex{},
		addresses: make(map[peer.ID]peer.AddrInfo),
	}
}

func NewAddress(node *core.IpfsNode) *Address {
	addr := defaultAddress()
	cfg, err := node.Repo.LinkConfig()
	if err != nil {
		return addr
	}
	fmt.Println("cache initialized")
	addr.cache = HashCacher(node.Repo.Path(), cfg.Address)
	return addr
}

func (a *Address) CheckPeerAddress(id peer.ID) (b bool) {
	a.lock.RLock()
	_, b = a.addresses[id]
	a.lock.RUnlock()
	return
}

func (a *Address) AddPeerAddress(addr peer.AddrInfo) (b bool) {
	a.lock.RLock()
	_, b = a.addresses[addr.ID]
	a.lock.RUnlock()
	if b {
		return !b
	}
	a.lock.Lock()
	_, b = a.addresses[addr.ID]
	if !b {
		a.addresses[addr.ID] = addr
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

func (a *Address) UpdatePeerAddress(new peer.AddrInfo) bool {
	address, b := a.GetAddress(new.ID)
	if !b {
		return a.AddPeerAddress(new)
	}

	mark := make(map[string]bool)
	for _, addr := range address.Addrs {
		mark[addr.String()] = true
	}

	for _, addr := range new.Addrs {
		if mark[addr.String()] {
			delete(mark, addr.String())
		}
	}

	if len(mark) == 0 {
		return false
	}

	a.lock.Lock()
	a.addresses[new.ID] = new
	a.lock.Unlock()
	return true
}

func (a *Address) LoadAddress(ctx context.Context) (<-chan peer.AddrInfo, error) {
	ai := make(chan peer.AddrInfo)
	go func() {
		defer close(ai)
		a.cache.Range(func(hash string, value string) bool {
			log.Infow("range node", "hash", hash, "value", value)
			var info peer.AddrInfo
			err := info.UnmarshalJSON([]byte(value))
			//err := json.Unmarshal([]byte(value), &ninfo)
			if err != nil {
				log.Errorw("load addr info failed", "err", err)
				return true
			}
			select {
			case <-ctx.Done():
				return false
			case ai <- info:
				return true
			}
		})
	}()
	return ai, nil
}

// SaveNode ...
func (a *Address) SaveAddress(ctx context.Context) (err error) {
	go func() {
		for _, id := range a.Peers() {
			select {
			case <-ctx.Done():
				return
			default:
				address, b := a.GetAddress(id)
				if !b {
					continue
				}
				err := a.cache.Store(id.Pretty(), address)
				if err != nil {
					return
				}
			}
		}
	}()
	return nil
}
