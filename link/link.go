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
const LinkPeers = "/link" + "/peers/" + Version
const LinkAddress = "/link" + "/address/" + Version

var protocols = []string{
	LinkPeers,
	LinkAddress,
}

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
	fmt.Println(l.node.Peerstore.GetProtocols(l.node.Identity))
	for {
		fmt.Println("all peers", l.node.Peerstore.PeersWithAddrs())

		for _, peer := range l.node.Peerstore.PeersWithAddrs() {

			//fmt.Println(l.node.Peerstore.AddProtocols(peer, LinkAddress, LinkPeers))
			s, err := l.node.PeerHost.NewStream(l.ctx, peer, LinkPeers)
			if err != nil {
				fmt.Println("found error:", err)
				continue
			}
			info := l.node.Peerstore.PeerInfo(peer)
			json, err := info.MarshalJSON()
			if err != nil {
				continue
			}
			s.Write(json)
			s.Write([]byte{'\n'})
			fmt.Println("send address:", info.String())

			s.Close()
		}
		time.Sleep(5 * time.Second)
	}

}

func (l *link) registerHandle() {
	l.node.PeerHost.SetStreamHandler(LinkPeers, func(stream network.Stream) {
		fmt.Println("link peer called")
		reader := bufio.NewReader(stream)
		defer stream.Close()
		ai := peer.AddrInfo{}
		for line, _, err := reader.ReadLine(); err == nil; {
			err := ai.UnmarshalJSON(line)
			if err != nil {
				continue
			}
			err = l.node.PeerHost.Connect(l.ctx, ai)
			if err != nil {
				continue
			}
			fmt.Println("remote addr", stream.Conn().RemoteMultiaddr())
			fmt.Println("connect to address", ai.String())
		}

	})
	l.node.PeerHost.SetStreamHandler(LinkAddress, func(stream network.Stream) {
		fmt.Println("link address called")
		fmt.Println(stream.Conn().RemoteMultiaddr())
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
	//fmt.Println(l.node.Peerstore.GetProtocols(l.node.Identity))
	//fmt.Println(l.node.PeerHost.Peerstore().GetProtocols(l.node.Identity))
	//if err := l.node.PeerHost.Peerstore().AddProtocols(l.node.Identity, protocols...); err != nil {
	//	return err
	//}
	//fmt.Println(l.node.Peerstore.GetProtocols(l.node.Identity))
	l.registerHandle()
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
