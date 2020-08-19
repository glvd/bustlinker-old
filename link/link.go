package link

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/glvd/bustlinker/core"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

type Linker interface {
	Start() error
	ListenAndServe(lis manet.Listener) error
}

type link struct {
	ctx        context.Context
	node       *core.IpfsNode
	connectors sync.Map
}

func (l *link) ListenAndServe(lis manet.Listener) error {
	for {
		conn, err := lis.Accept()
		if err != nil {
			continue
		}
		err = l.NewConn(conn)
		if err == nil {
			continue
		}
		//no callback closed
		err = conn.Close()
		if err != nil {
			log.Errorw("accept conn error", "err", err)
		}
	}
}

func (l *link) NewConn(conn net.Conn) error {
	return nil
}

func (l link) Start() error {
	fmt.Println("Link start")
	//api, err := coreapi.NewCoreAPI(l.node)
	//if err != nil {
	//	return err
	//}
	cma := make(chan peer.AddrInfo)
	//peerCheck := make(map[peer.ID]bool)
	go func() {
		for info := range cma {
			fmt.Println(info.String())
		}
	}()
	for {
		time.Sleep(5 * time.Second)
		//for _, p := range l.node.Peerstore.Peers() {
		peers, err := l.node.Routing.FindPeer(context.TODO(), "QmfSQ3g19LHDtAftSUJ5Nph6jgVxYcpDL6wpaXHQRRrqXb")
		fmt.Println("addrs", peers)
		if err != nil {
			continue
		}
		//l.node.
		////if !peerCheck[p] {
		peers2 := l.node.Peerstore.Addrs(peers.ID)
		fmt.Println(peers.ID, "addrs", peers)
		time.Sleep(15 * time.Second)
		for _, p2 := range peers2 {
			relay2, _ := p2.ValueForProtocol(multiaddr.P_P2P)
			rID2, err := peer.IDFromString(relay2)
			if err != nil {
				log.Debugf("failed to parse relay ID in address %s: %s", relay2, err)
				continue
			}
			go func(ctx context.Context, id peer.ID) {
				m := l.node.Peerstore.PeerInfo(id)
				select {
				case <-ctx.Done():
					break
				case cma <- m:
				}
			}(l.ctx, rID2)
		}

		//}
		//}

	}
	return nil
}

func New(ctx context.Context, node *core.IpfsNode) Linker {
	return &link{
		ctx:  ctx,
		node: node,
	}
}

var _ Linker = &link{}
