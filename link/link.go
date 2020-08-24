package link

import (
	"bufio"
	"context"
	"fmt"
	"github.com/glvd/bustlinker/core"
	"github.com/glvd/bustlinker/core/coreapi"
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

var NewLine = []byte{'\n'}

type Linker interface {
	Start() error
	ListenAndServe() error
}

type link struct {
	ctx       context.Context
	node      *core.IpfsNode
	addresses *Address

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

func (l *link) Syncing() {
	for {
		wg := &sync.WaitGroup{}

		for _, conn := range l.node.PeerHost.Network().Conns() {
			if l.node.Identity == conn.RemotePeer() {
				continue
			}
			wg.Add(1)
			go l.getPeerAddress(wg, conn.RemotePeer())
		}
		wg.Wait()
		time.Sleep(15 * time.Second)
	}
}

func checkAddrExist(addrs []multiaddr.Multiaddr, addr multiaddr.Multiaddr) bool {
	for i := range addrs {
		if addr.Equal(addrs[i]) {
			return true
		}
	}
	return false
}

func (l *link) registerHandle() {
	l.node.PeerHost.SetStreamHandler(LinkPeers, func(stream network.Stream) {
		fmt.Println("link peer called")
		var err error
		defer stream.Close()
		//addrs := filterAddrs(stream.Conn().RemoteMultiaddr(), l.node.Peerstore.Addrs(stream.Conn().RemotePeer()))
		//l.node.Peerstore.AddAddr()
		remoteID := stream.Conn().RemotePeer()
		fmt.Println("id:", remoteID, stream.Conn().RemoteMultiaddr().String())
		if !checkAddrExist(l.node.Peerstore.Addrs(remoteID), stream.Conn().RemoteMultiaddr()) {
			l.node.Peerstore.AddAddr(remoteID, stream.Conn().RemoteMultiaddr(), 7*24*time.Hour)
		}

		conns := l.node.PeerHost.Network().Conns()
		for _, conn := range conns {
			fmt.Println("remote addr", conn.RemoteMultiaddr())
			info := l.node.Peerstore.PeerInfo(conn.RemotePeer())
			//l.node.Peerstore.ClearAddrs(pid)
			//info := l.node.Peerstore.PeerInfo(pid)
			json, _ := info.MarshalJSON()
			_, err = stream.Write(json)
			if err != nil {
				fmt.Println("err", err)
				return
			}
			_, _ = stream.Write(NewLine)
			fmt.Println("send addresses:", info.String())
		}
	})
	l.node.PeerHost.SetStreamHandler(LinkAddress, func(stream network.Stream) {
		fmt.Println("link addresses called")
		fmt.Println(stream.Conn().RemoteMultiaddr())
	})
}

func (l *link) getStream(id peer.ID) (network.Stream, error) {
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
	s, err = l.node.PeerHost.NewStream(l.ctx, id, LinkPeers)
	if err != nil {
		return nil, err
	}
	//s.SetProtocol(LinkPeers)
	//l.streamLock.Lock()
	//_, b = l.streams[id]
	//if !b {
	//	l.streams[id] = s
	//}
	//l.streamLock.Unlock()
	return s, nil
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
	go l.Syncing()
	return nil
}

func (l *link) getPeerAddress(wg *sync.WaitGroup, pid peer.ID) {
	defer wg.Done()
	s, err := l.getStream(pid)
	if err != nil {
		fmt.Println("found error:", err)
		return
	}
	defer s.Close()
	//all, err := ioutil.ReadAll(s)
	reader := bufio.NewReader(s)
	for line, _, err := reader.ReadLine(); err == nil; {
		fmt.Println("json:", string(line))
		ai := peer.AddrInfo{}
		err := ai.UnmarshalJSON(line)
		if err != nil {
			fmt.Println("unmarlshal json:", string(line), err)
			return
		}
		fmt.Println("received addresses", ai.String(), len(ai.Addrs))
		if ai.ID == l.node.Identity {
			continue
		}
		l.AddPeerAddress(ai.ID, ai)
		fmt.Println("sleep for next")
		time.Sleep(5 * time.Second)
	}
}

func (l *link) AddPeerAddress(id peer.ID, ai peer.AddrInfo) {
	//l.node.Peering.AddPeer(ai)
	if l.addresses.AddPeerAddress(id, ai) {
		api, err := coreapi.NewCoreAPI(l.node)
		if err != nil {
			fmt.Println("err", err)
			return
		}
		err = api.Swarm().Connect(l.ctx, ai)
		if err != nil {
			fmt.Println("err", err)
			return
		}
		fmt.Println("connect success:", ai.String())
	}
}

func (l *link) RegisterAddresses(address *Address) {
	l.addresses = address
}

func New(ctx context.Context, root string, node *core.IpfsNode) Linker {
	config, err := node.Repo.Config()
	if err != nil {
		return nil
	}
	return &link{
		ctx:       ctx,
		node:      node,
		addresses: NewAddress(config.Link.Hash),
		//streams:     make(map[peer.ID]network.Stream),
		//streamLock:  &sync.RWMutex{},
		//Listener:    ,
	}
}

var _ Linker = &link{}
