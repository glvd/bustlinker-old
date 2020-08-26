package link

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/glvd/bustlinker/core"
	"github.com/glvd/bustlinker/core/coreapi"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/multiformats/go-multiaddr"
	"sync"
	"time"
)

const Version = "0.0.1"
const LinkPeers = "/link" + "/peers/" + Version
const LinkAddress = "/link" + "/address/" + Version
const LinkHash = "/link" + "/hash/" + Version

var protocols = []string{
	LinkPeers,
	LinkAddress,
}

var NewLine = []byte{'\n'}

type Linker interface {
	Start() error
}

type link struct {
	ctx         context.Context
	node        *core.IpfsNode
	failedCount map[peer.ID]int64
	failedLock  *sync.RWMutex
	pins        []string
	pinsLock    *sync.RWMutex
	addresses   *PeerCache
	hashes      *HashCache
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
			go l.syncPin()
			wg.Add(1)
			go l.getPeerAddress(wg, conn)
			wg.Add(1)
			go l.getRemoteHash(wg, conn)
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

func (l *link) newLinkPeersHandle() (protocol.ID, func(stream network.Stream)) {
	return LinkPeers, func(stream network.Stream) {
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
	}
}

func (l *link) getPins() []string {
	var pins []string
	l.pinsLock.RLock()
	if l.pins == nil {
		return pins
	}
	pins = make([]string, len(l.pins))
	copy(pins, l.pins)
	l.pinsLock.RUnlock()
	return pins
}

func (l *link) addPin(pin string) {
	l.pinsLock.Lock()
	l.pins = append(l.pins, pin)
	l.pinsLock.Unlock()
}

func (l *link) setPins(pins []string) {
	p := make([]string, len(pins))
	copy(p, pins)
	l.pinsLock.Lock()
	l.pins = p
	l.pinsLock.Unlock()
}

func (l *link) newLinkHashHandle() (protocol.ID, func(stream network.Stream)) {
	return LinkHash, func(stream network.Stream) {
		fmt.Println("link hash called")
		var err error
		defer stream.Close()
		for _, peer := range l.getPins() {
			_, err = stream.Write([]byte(peer))
			if err != nil {
				log.Debugw("stream write error", "error", err)
				return
			}
			_, err = stream.Write(NewLine)
			if err != nil {
				log.Debugw("stream write error", "error", err)
				return
			}
		}
	}
}

func (l *link) registerHandle() {
	l.node.PeerHost.SetStreamHandler(l.newLinkPeersHandle())
	l.node.PeerHost.SetStreamHandler(l.newLinkHashHandle())
}

func (l *link) Start() error {
	fmt.Println("Link start")
	l.registerHandle()
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	address, err := l.addresses.LoadAddress(ctx)
	if err != nil {
		return err
	}
	l.addresses.UpdatePeerAddress(<-address)
	go l.Syncing()
	return nil
}

func (l *link) getPeerAddress(wg *sync.WaitGroup, conn network.Conn) {
	defer wg.Done()
	s, err := l.node.PeerHost.NewStream(l.ctx, conn.RemotePeer(), LinkPeers)
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
			if err := l.UpdatePeerAddress(ai); err != nil {
				log.Error("update peer address failed:", err)
			}
		}
	}
}

func (l *link) getAddCount(id peer.ID) int64 {
	count := int64(0)
	l.failedLock.Lock()
	count, l.failedCount[id] = l.failedCount[id], l.failedCount[id]+1
	l.failedLock.Unlock()
	return count
}

func (l *link) UpdatePeerAddress(ai peer.AddrInfo) error {
	stream, err := l.node.PeerHost.NewStream(l.ctx, ai.ID, LinkPeers)
	if err == nil {
		stream.Close()
		return nil
	}
	log.Debug("stream connect failed:", err)
	config, err := l.node.Repo.Config()
	if err != nil {
		return err
	}

	if l.addresses.UpdatePeerAddress(ai) {
		count := l.getAddCount(ai.ID)
		if count > config.Link.MaxAttempts {
			return errors.New("connect failed max")
		}
		api, err := coreapi.NewCoreAPI(l.node)
		if err != nil {
			return err
		}
		err = api.Swarm().Connect(l.ctx, ai)
		if err != nil {
			return err
		}
		fmt.Println("connect success:", ai.String())
	}
	return nil
}

func (l *link) getRemoteHash(wg *sync.WaitGroup, conn network.Conn) {
	defer wg.Done()
	id := conn.RemotePeer()
	s, err := l.node.PeerHost.NewStream(l.ctx, id, LinkHash)
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
			l.UpdateHash(string(line), id)
		}
	}
}

func (l *link) UpdateHash(hash string, id peer.ID) {
	l.hashes.Add(hash, id)
}

func (l *link) syncPin() {
	api, err := coreapi.NewCoreAPI(l.node)
	if err != nil {
		return
	}
	ls, err := api.Pin().Ls(l.ctx, options.Pin.Ls.Recursive())
	if err != nil {
		return
	}

	for pin := range ls {
		fmt.Println(pin.Path().String())
		l.addPin(pin.Path().String())
	}
}

func New(ctx context.Context, node *core.IpfsNode) Linker {
	return &link{
		ctx:         ctx,
		node:        node,
		failedCount: make(map[peer.ID]int64),
		failedLock:  &sync.RWMutex{},
		pins:        nil,
		pinsLock:    &sync.RWMutex{},
		addresses:   NewAddress(node),
		hashes:      NewHash(node),
	}
}

var _ Linker = &link{}
