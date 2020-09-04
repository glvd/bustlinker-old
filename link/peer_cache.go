package link

import (
	"context"
	"fmt"
	"github.com/glvd/bustlinker/core"
	"github.com/libp2p/go-libp2p-core/peer"
	"sync"
)

const peerName = "address"

type PeerCache struct {
	lock      *sync.RWMutex
	addresses map[peer.ID]peer.AddrInfo
	cache     Cacher
}

func peerCache() *PeerCache {
	return &PeerCache{
		lock:      &sync.RWMutex{},
		addresses: make(map[peer.ID]peer.AddrInfo),
	}
}

func NewAddress(node *core.IpfsNode) *PeerCache {
	cache := peerCache()
	cfg, err := node.Repo.LinkConfig()
	if err != nil {
		return cache
	}
	fmt.Println("peer cache initialized")

	cache.cache = NewCache(cfg.Address, node.Repo.Path(), peerName)
	return cache
}

func (c *PeerCache) CheckPeerAddress(id peer.ID) (b bool) {
	c.lock.RLock()
	_, b = c.addresses[id]
	c.lock.RUnlock()
	return
}

func (c *PeerCache) AddPeerAddress(addr peer.AddrInfo) (b bool) {
	c.lock.RLock()
	_, b = c.addresses[addr.ID]
	c.lock.RUnlock()
	if b {
		return !b
	}
	c.lock.Lock()
	_, b = c.addresses[addr.ID]
	if !b {
		c.addresses[addr.ID] = addr
	}
	c.lock.Unlock()
	return !b
}

func (c *PeerCache) GetAddress(id peer.ID) (ai peer.AddrInfo, b bool) {
	c.lock.RLock()
	ai, b = c.addresses[id]
	c.lock.RUnlock()
	return ai, b
}

func (c *PeerCache) Peers() (ids []peer.ID) {
	c.lock.RLock()
	for id := range c.addresses {
		ids = append(ids, id)
	}
	c.lock.RUnlock()
	return
}

func (c *PeerCache) UpdatePeerAddress(new peer.AddrInfo) bool {
	address, b := c.GetAddress(new.ID)
	if !b {
		return c.AddPeerAddress(new)
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

	c.lock.Lock()
	c.addresses[new.ID] = new
	c.lock.Unlock()
	return true
}

func (c *PeerCache) LoadAddress(ctx context.Context) (<-chan peer.AddrInfo, error) {
	ai := make(chan peer.AddrInfo)
	go func() {
		defer close(ai)
		c.cache.Range(func(hash string, value string) bool {
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
func (c *PeerCache) SaveAddress(ctx context.Context) (err error) {
	go func() {
		for _, id := range c.Peers() {
			select {
			case <-ctx.Done():
				return
			default:
				address, b := c.GetAddress(id)
				if !b {
					continue
				}
				err := c.cache.Store(id.Pretty(), address)
				if err != nil {
					return
				}
			}
		}
	}()
	return nil
}
