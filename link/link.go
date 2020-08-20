package link

import (
	"bufio"
	"context"
	"fmt"
	"github.com/glvd/bustlinker/core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"net"
	"sync"
	"time"
)

const Version = "0.0.1"
const LinkPeers = "Link/" + Version + "/Peers"
const LinkAddress = "Link/" + Version + "/Address"

type Linker interface {
	Start() error
	ListenAndServe() error
}

type link struct {
	ctx  context.Context
	node *core.IpfsNode

	address     map[peer.ID]multiaddr.Multiaddr
	addressLock *sync.RWMutex
}

func (l *link) ListenAndServe() error {
	return nil
}

func (l *link) SyncPeers() {

	for {
		fmt.Println("all peers", l.node.Peerstore.PeersWithAddrs())
		for _, peer := range l.node.Peerstore.PeersWithAddrs() {
			s, err := l.node.PeerHost.NewStream(l.ctx, peer, LinkAddress)
			if err != nil {
				fmt.Println("found error", err)
				continue
			}
			addrs := l.node.Peerstore.Addrs(peer)
			for _, addr := range addrs {
				s.Write(addr.Bytes())
				s.Write([]byte{'\n'})
				fmt.Println("send address:", addr.String())
			}
			s.Close()
		}
		time.Sleep(5 * time.Second)
	}

}

func (l *link) registerHandle() {
	l.node.PeerHost.SetStreamHandler(LinkPeers, func(stream network.Stream) {
		reader := bufio.NewReader(stream)
		defer stream.Close()
		for line, _, err := reader.ReadLine(); err == nil; {
			bytes, err := multiaddr.NewMultiaddrBytes(line)
			if err != nil {
				continue
			}
			fmt.Println("address", bytes.String())
		}

	})
	l.node.PeerHost.SetStreamHandler(LinkAddress, func(stream network.Stream) {
		stream.Conn().RemoteMultiaddr()
	})
}

func (l *link) AddPeerAddress(id peer.ID, addrs multiaddr.Multiaddr) {
	l.addressLock.Lock()
	l.address[id] = addrs
	l.addressLock.Unlock()
}

func (l *link) NewConn(conn net.Conn) error {
	return nil
}

func (l *link) Start() error {
	fmt.Println("Link start")
	go l.SyncPeers()
	return nil
}

func New(ctx context.Context, node *core.IpfsNode) Linker {
	return &link{
		ctx:         ctx,
		node:        node,
		address:     make(map[peer.ID]multiaddr.Multiaddr),
		addressLock: &sync.RWMutex{},
	}
}

var _ Linker = &link{}
