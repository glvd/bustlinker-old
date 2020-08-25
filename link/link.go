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

	scdt.Listener
}

func (l *link) ListenAndServe() error {
	return nil
}

func (l *link) syncPeers() {
	//listener, err := scdt.NewListener(l.node.Identity.String())
	//if err != nil {
	//	return
	//}
	//l.Listener = listener
	//config, err := l.node.Repo.LinkConfig()
	//if err != nil {
	//	return
	//}

	//for _, address := range config.Addresses {
	//	ma, err := multiaddr.NewMultiaddr(address)
	//	if err != nil {
	//		continue
	//	}
	//	nw, ip, err := manet.DialArgs(ma)
	//	if err != nil {
	//		return
	//	}
	//	listen, err := reuse.Listen(nw, ip)
	//	if err != nil {
	//		return
	//	}
	//	l.Listener.Listen(nw, listen)
	//}

}

func (l *link) Syncing() {
	for {
		wg := &sync.WaitGroup{}
		for _, conn := range l.node.PeerHost.Network().Conns() {
			if l.node.Identity == conn.RemotePeer() {
				continue
			}
			wg.Add(1)
			go l.getPeerAddress(wg, conn)
		}
		wg.Wait()
		fmt.Println("Wait for next loop")
		time.Sleep(30 * time.Second)
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
		remoteID := stream.Conn().RemotePeer()
		if !checkAddrExist(l.node.Peerstore.Addrs(remoteID), stream.Conn().RemoteMultiaddr()) {
			l.node.Peerstore.AddAddr(remoteID, stream.Conn().RemoteMultiaddr(), 7*24*time.Hour)
		}

		peers := l.node.PeerHost.Network().Peers()
		fmt.Println("total:", len(peers))
		for _, peer := range peers {
			info := l.node.Peerstore.PeerInfo(peer)
			json, _ := info.MarshalJSON()
			_, err = stream.Write(json)
			if err != nil {
				log.Debugw("stream write error", "error", err)
				return
			}
			_, err = stream.Write(NewLine)
			if err != nil {
				log.Debugw("stream write error", "error", err)
				return
			}
			fmt.Println("to:", remoteID, stream.Conn().RemoteMultiaddr().String(), "send addresses:", info.String())
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

func (l *link) getPeerAddress(wg *sync.WaitGroup, conn network.Conn) {
	defer wg.Done()
	s, err := l.getStream(conn.RemotePeer())
	if err != nil {
		return
	}
	defer s.Close()

	reader := bufio.NewReader(s)
	for {
		select {
		case <-l.ctx.Done():
			return
		default:
			line, _, err := reader.ReadLine()
			if err != nil {
				return
			}
			ai := peer.AddrInfo{}
			err = ai.UnmarshalJSON(line)
			if err != nil {
				fmt.Println("unmarlshal json:", string(line), err)
				return
			}
			if ai.ID == l.node.Identity {
				continue
			}
			fmt.Println("from:", conn.RemotePeer().Pretty(), "received new addresses:", ai.String(), len(ai.Addrs))
			l.UpdatePeerAddress(ai)
		}
	}
}

func (l *link) AddPeerAddress(ai peer.AddrInfo) bool {
	return l.addresses.AddPeerAddress(ai)
}

func (l *link) UpdatePeerAddress(ai peer.AddrInfo) {
	stream, err := l.getStream(ai.ID)
	fmt.Println("update",ai.ID.Pretty(),"err",err)
	if err == nil {
		stream.Close()
		return
	}
	fmt.Println("err", err)
	if l.addresses.UpdatePeerAddress(ai) {
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

func New(ctx context.Context, node *core.IpfsNode) Linker {
	return &link{
		ctx:       ctx,
		node:      node,
		addresses: NewAddress(node),
	}
}

var _ Linker = &link{}
