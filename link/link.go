package link

import (
	"bufio"
	"context"
	"fmt"
	"github.com/glvd/bustlinker/core"
	"github.com/godcong/scdt"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/portmapping/go-reuse"
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
	ctx         context.Context
	node        *core.IpfsNode
	addresses   map[peer.ID]peer.AddrInfo
	addressLock *sync.RWMutex

	//streams    map[peer.ID]network.Stream
	//streamLock *sync.RWMutex

	scdt.Listener
}

func (l *link) ListenAndServe() error {
	return nil
}

func (l *link) syncPeers() {
	listener, err := scdt.NewListener(l.node.Identity.String())
	if err != nil {
		return
	}
	l.Listener = listener
	config, err := l.node.Repo.LinkConfig()
	if err != nil {
		return
	}
	//api, err := coreapi.NewCoreAPI(l.node)
	//if err != nil {
	//	return
	//}

	for _, address := range config.Addresses {
		ma, err := multiaddr.NewMultiaddr(address)
		if err != nil {
			continue
		}
		nw, ip, err := manet.DialArgs(ma)
		if err != nil {
			return
		}
		listen, err := reuse.Listen(nw, ip)
		if err != nil {
			return
		}
		l.Listener.Listen(nw, listen)
	}

}

func (l *link) SyncPeers() {
	//api, err := coreapi.NewCoreAPI(l.node)
	//if err != nil {
	//	return
	//}

	for {
		for _, pid := range l.node.Peerstore.PeersWithAddrs() {
			if l.node.Identity == pid {
				continue
			}
			go l.getPeerAddress(pid)

		}
		time.Sleep(5 * time.Second)
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
			//if err != nil {
			//	stream.Reset()
			//} else {
			//stream.Close()
			//}
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
			fmt.Println("send addresses:", info.String())
		}

	})
	l.node.PeerHost.SetStreamHandler(LinkAddress, func(stream network.Stream) {
		fmt.Println("link addresses called")
		fmt.Println(stream.Conn().RemoteMultiaddr())
	})
}

func (l *link) AddPeerAddress(id peer.ID, addrs peer.AddrInfo) {
	l.addressLock.Lock()
	l.addresses[id] = addrs
	l.addressLock.Unlock()
}

func (l *link) AddAddress(id peer.ID, addrs peer.AddrInfo) {
	l.addressLock.Lock()
	l.addresses[id] = addrs
	l.addressLock.Unlock()
}

func (l *link) GetStream(id peer.ID) (network.Stream, error) {
	var s network.Stream
	//var b bool
	var err error
	//l.streamLock.RLock()
	//s, b = l.streams[id]
	//l.streamLock.RUnlock()
	//
	//if b {
	//	return s, nil
	//}
	s, err = l.node.PeerHost.Network().NewStream(l.ctx, id)
	if err != nil {
		return nil, err
	}
	s.SetProtocol(LinkPeers)
	//l.streamLock.Lock()
	//_, b = l.streams[id]
	//if !b {
	//	l.streams[id] = s
	//}
	//l.streamLock.Unlock()
	return s, nil
}

func (l *link) CheckPeerAddress(id peer.ID) (b bool) {
	l.addressLock.RLock()
	_, b = l.addresses[id]
	l.addressLock.RUnlock()
	return
}

func (l *link) Conn(conn scdt.Connection) error {
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

func (l *link) getPeerAddress(pid peer.ID) {
	s, err := l.GetStream(pid)
	if err != nil {
		fmt.Println("found error:", err)
		return
	}
	defer s.Close()
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
		if l.CheckPeerAddress(ai.ID) {
			continue
		}
		fmt.Println("received addresses", ai.String())
		//err = api.Swarm().Connect(l.ctx, ai)
		//if err != nil {
		//	fmt.Println("connect error:", err)
		//	continue
		//}
		l.AddPeerAddress(ai.ID, ai)
		//fmt.Println("connected to addresses", ai.String())
	}
}

func New(ctx context.Context, node *core.IpfsNode) Linker {
	return &link{
		ctx:         ctx,
		node:        node,
		addresses:   make(map[peer.ID]peer.AddrInfo),
		addressLock: &sync.RWMutex{},
		//streams:     make(map[peer.ID]network.Stream),
		//streamLock:  &sync.RWMutex{},
		//Listener:    ,
	}
}

var _ Linker = &link{}
