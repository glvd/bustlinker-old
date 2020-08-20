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

		for _, pid := range l.node.Peerstore.PeersWithAddrs() {
			if l.node.Identity == pid {
				continue
			}
			//fmt.Println(l.node.Peerstore.AddProtocols(pid, LinkAddress, LinkPeers))
			s, err := l.node.PeerHost.NewStream(l.ctx, pid, LinkPeers)
			if err != nil {
				fmt.Println("found error:", err)
				continue
			}
			reader := bufio.NewReader(s)
			ai := peer.AddrInfo{}
			for line, _, err := reader.ReadLine(); err == nil; {
				err := ai.UnmarshalJSON(line)
				if err != nil {
					fmt.Println("unmarlshal json:", err)
					continue
				}
				if ai.ID == l.node.Identity {
					continue
				}
				fmt.Println("connect to address", ai.String())
				err = l.node.PeerHost.Connect(l.ctx, ai)
				if err != nil {
					fmt.Println("connect error:", err)
					continue
				}
				fmt.Println("connected to address", ai.String())
				time.Sleep(5 * time.Second)
			}

			time.Sleep(15 * time.Second)
		}
		time.Sleep(30 * time.Second)
	}
}

func filterAddrs(addr multiaddr.Multiaddr, addrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
	v := map[multiaddr.Multiaddr]bool{
		addr: true,
	}
	for i := range addrs {
		v[addrs[i]] = true
	}
	var retAddrs []multiaddr.Multiaddr
	for i := range v {
		retAddrs = append(retAddrs, i)
	}
	return retAddrs
}

func (l *link) registerHandle() {
	l.node.PeerHost.SetStreamHandler(LinkPeers, func(stream network.Stream) {
		fmt.Println("link peer called")
		var err error
		defer func() {
			if err != nil {
				stream.Reset()
			} else {
				stream.Close()
			}
		}()
		addrs := filterAddrs(stream.Conn().RemoteMultiaddr(), l.node.Peerstore.Addrs(stream.Conn().RemotePeer()))
		l.node.Peerstore.SetAddrs(stream.Conn().RemotePeer(), addrs, 7*24*time.Hour)
		fmt.Println("remote addr", stream.Conn().RemoteMultiaddr())
		for _, pid := range l.node.Peerstore.PeersWithAddrs() {
			info := l.node.Peerstore.PeerInfo(pid)
			json, _ := info.MarshalJSON()
			_, err = stream.Write(json)
			_, err = stream.Write([]byte{'\n'})
			if err != nil {
				return
			}
			fmt.Println("send address:", info.String())
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
