package link

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"time"

	"github.com/glvd/bustlinker/core"
)

type Linker interface {
	Start() error
}

type link struct {
	ctx     context.Context
	node    *core.IpfsNode
	manager Manager
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

func New(ctx context.Context, node *core.IpfsNode) (Linker, error) {

	return &link{
		ctx:     ctx,
		node:    node,
		manager: nil,
	}, nil
}

var _ Linker = &link{}
