package link

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/panjf2000/ants/v2"
	"net"
	"sync"
	"time"

	"github.com/glvd/accipfs/basis"
	"github.com/glvd/accipfs/config"
	"github.com/glvd/accipfs/core"
	"github.com/godcong/scdt"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	mnet "github.com/multiformats/go-multiaddr-net"
	"go.uber.org/atomic"
)

type Manager interface {
}

type manager struct {
	scdt.Listener
	loopOnce        *sync.Once
	cfg             *config.Config
	t               *time.Ticker
	currentTS       int64
	ts              int64
	local           core.SafeLocalData
	nodePool        *ants.PoolWithFunc
	currentNodes    *atomic.Int32
	connectNodes    sync.Map
	disconnectNodes sync.Map
	nodes           Cacher //all node caches
	hashNodes       Cacher //hash cache nodes
	RequestLD       func() ([]string, error)
	gc              *atomic.Bool
	addrCB          func(info peer.AddrInfo) error
}

//var _ core.NodeManager = &manager{}

// InitManager ...
func InitManager(cfg *config.Config) (Manager, error) {
	if cfg.Node.BackupSeconds == 0 {
		cfg.Node.BackupSeconds = 30
	}
	data := core.DefaultLocalData()
	m := &manager{
		cfg:      cfg,
		loopOnce: &sync.Once{},
		//initLoad:  atomic.NewBool(false),
		//path:      filepath.Join(cfg.Path, _nodes),
		//expPath:   filepath.Join(cfg.Path, _expNodes),
		nodes:     NodeCacher(cfg),
		hashNodes: HashCacher(cfg),
		local:     data.Safe(),
		t:         time.NewTicker(cfg.Node.BackupSeconds * time.Second),
	}
	m.nodePool = mustPool(cfg.Node.PoolMax, m.mainProc)
	return m, nil
}

//// NodeAPI ...
//func (m *manager) NodeAPI() core.NodeAPI {
//	return m
//}

func mustPool(size int, pf func(v interface{})) *ants.PoolWithFunc {
	if size == 0 {
		size = ants.DefaultAntsPoolSize
	}
	withFunc, err := ants.NewPoolWithFunc(size, pf)
	if err != nil {
		panic(err)
	}
	return withFunc
}

// SaveNode ...
func (m *manager) SaveNode() (err error) {
	m.connectNodes.Range(func(key, value interface{}) bool {
		keyk, keyb := key.(string)
		valv, valb := value.(core.Node)
		log.Infow("store", "key", keyk, "keyOK", keyb, "value", valv, "valOK", valb)
		if !valb || !keyb || keyk == "" {
			return true
		}
		info, err := valv.GetInfo()
		if err != nil {
			return true
		}

		err = m.nodes.Store(keyk, info)
		if err != nil {
			log.Errorw("failed store", "err", err)
			return true
		}
		fmt.Println("node", keyk, "was stored")
		return true
	})
	//return the last err
	return
}

// Link ...
func (m *manager) Link(ctx context.Context, req *core.NodeLinkReq) (*core.NodeLinkResp, error) {
	fmt.Printf("connect info:%+v\n", req.Addrs)
	if !m.local.Data().Initialized {
		return &core.NodeLinkResp{}, errors.New("you are not ready for connection")
	}
	if req.Timeout == 0 {
		req.Timeout = 5 * time.Second
	}
	d := mnet.Dialer{
		Dialer: net.Dialer{
			Timeout: req.Timeout,
		},
	}
	var infos []core.NodeInfo
	if req.ByID {
		for _, name := range req.Names {
			var info core.NodeInfo
			err := m.nodes.Load(name, &info)
			if err != nil {
				continue
			}
			log.Infow("info", "info", info.JSON())
			for _, multiaddr := range info.AddrInfo.GetAddrs() {
				fmt.Println("connect to", multiaddr.String())
				dial, err := d.Dial(multiaddr)
				if err != nil {
					fmt.Printf("link failed(%v)\n", err)
					continue
				}
				conn, err := m.newConn(dial)
				if err != nil {
					continue
				}
				id := conn.ID()
				getNode, b := m.GetNode(id)
				if b {
					conn = getNode
				} else {
					//use conn
				}
				infos = append(infos, info)
				break
			}
		}
	} else {
		for _, addr := range req.Addrs {
			multiaddr, err := ma.NewMultiaddr(addr)
			if err != nil {
				fmt.Printf("parse addr(%v) failed(%v)\n", addr, err)
				continue
			}
			fmt.Println("connect to", multiaddr.String())
			dial, err := d.Dial(multiaddr)
			if err != nil {
				fmt.Printf("link failed(%v)\n", err)
				continue
			}
			conn, err := m.newConn(dial)
			if err != nil {
				return &core.NodeLinkResp{}, err
			}
			id := conn.ID()
			getNode, b := m.GetNode(id)
			if b {
				conn = getNode
			} else {
				//use conn
			}
			info, err := conn.GetInfo()
			if err != nil {
				return &core.NodeLinkResp{}, err
			}
			infos = append(infos, info)
		}
	}
	return &core.NodeLinkResp{
		NodeInfos: infos,
	}, nil
}

// Unlink ...
func (m *manager) Unlink(ctx context.Context, req *core.NodeUnlinkReq) (*core.NodeUnlinkResp, error) {
	if len(req.Peers) == 0 {
		return &core.NodeUnlinkResp{}, nil
	}
	for i := range req.Peers {
		m.connectNodes.Delete(req.Peers[i])
	}
	return &core.NodeUnlinkResp{}, nil
}

// NodeAddrInfo ...
func (m *manager) NodeAddrInfo(ctx context.Context, req *core.AddrReq) (*core.AddrResp, error) {
	load, ok := m.connectNodes.Load(req.ID)
	if !ok {
		return &core.AddrResp{}, fmt.Errorf("node not found id(%s)", req.ID)
	}
	v, b := load.(core.Node)
	if !b {
		return &core.AddrResp{}, fmt.Errorf("transfer to node failed id(%s)", req.ID)
	}
	panic("//todo")
	fmt.Print(v)
	return nil, nil
}

// List ...
func (m *manager) List(ctx context.Context, req *core.NodeListReq) (*core.NodeListResp, error) {
	//todo:need optimization
	//nodes := make(map[string]core.NodeInfo)
	//m.Range(func(key string, node core.Node) bool {
	//	info, err := node.GetInfo()
	//	if err != nil {
	//		return true
	//	}
	//	nodes[key] = info
	//	return true
	//})
	return &core.NodeListResp{Nodes: m.local.Data().Nodes}, nil
}

// Local ...
func (m *manager) Local() core.SafeLocalData {
	return m.local
}

// LoadNode ...
func (m *manager) LoadNode() error {
	m.nodes.Range(func(hash string, value string) bool {
		log.Infow("range node", "hash", hash, "value", value)
		var ninfo core.NodeInfo
		err := ninfo.Unmarshal([]byte(value))
		//err := json.Unmarshal([]byte(value), &ninfo)
		if err != nil {
			log.Errorw("load addr info failed", "err", err)
			return true
		}
		for multiaddr := range ninfo.Addrs {
			fmt.Println("connect node with addresses:", multiaddr.String())
			connectNode, err := MultiDial(multiaddr, 0)
			if err != nil {
				continue
			}
			_, err = m.Conn(connectNode)
			if err != nil {
				continue
			}
			//m.connectNodes.SaveNode(hash, connectNode)
			return true
		}
		return true
	})
	//start loop after first load
	m.loopOnce.Do(func() {
		go m.loop()
	})

	return nil
}

// StateEx State Examination checks the node status
func (m *manager) StateEx(id string, f func(node core.Node) bool) {
	if f == nil {
		return
	}
	node, ok := m.connectNodes.Load(id)
	if ok {
		if f(node.(core.Node)) {
			m.connectNodes.Delete(id)
			m.disconnectNodes.Store(id, node)
		}
	}

	exp, ok := m.disconnectNodes.Load(id)
	if ok {
		if f(exp.(core.Node)) {
			m.disconnectNodes.Delete(id)
			m.connectNodes.Store(id, exp)
		}
	}
}

// Range ...
func (m *manager) Range(f func(key string, node core.Node) bool) {
	m.connectNodes.Range(func(key, value interface{}) bool {
		k, b1 := key.(string)
		n, b2 := value.(core.Node)
		if !b1 || !b2 {
			return true
		}
		if f != nil {
			return f(k, n)
		}
		return false
	})
}

// Push ...
func (m *manager) Push(node core.Node) {
	m.ts = time.Now().Unix()
	m.connectNodes.Store(node.ID(), node)
}

// save nodes
func (m *manager) loop() {
	for {
		<-m.t.C
		log.Infow("backup connect nodes")
		if m.ts != m.currentTS {
			if err := m.SaveNode(); err != nil {
				continue
			}
			m.currentTS = m.ts
		}
	}
}

// newConn ...
func (m *manager) newConn(c net.Conn) (core.Node, error) {
	acceptNode, err := CoreNode(c, m.local)
	if err != nil {
		return nil, err
	}

	m.nodePool.Invoke(acceptNode)
	return acceptNode, nil
}

func (m *manager) mainProc(v interface{}) {
	n, b := v.(core.Node)
	if !b {
		return
	}
	pushed := false
	id := n.ID()
	fmt.Println("user connect:", id)

	if id == "" {
		//wait remote client get base info
		time.Sleep(3 * time.Second)
		_ = n.SendConnected()
		return
	}
	defer func() {
		fmt.Println("id", id, "was exit")
		//wait client close itself
		time.Sleep(500 * time.Millisecond)
		n.Close()
		if pushed {
			m.connectNodes.Delete(id)
			m.local.Update(func(data *core.LocalData) {
				delete(data.Nodes, id)
			})
		}
	}()
	old, loaded := m.connectNodes.Load(id)
	log.Infow("new connection", "new", n.ID(), "isload", loaded)
	if loaded {
		nbase := old.(core.Node)
		if !nbase.IsClosed() {
			if n.Addrs() != nil {
				nbase.AppendAddr(n.Addrs()...)
			}
			//wait remote client get base info
			time.Sleep(3 * time.Second)
			_ = n.SendConnected()
			return
		}
	}
	//get remote node info
	info, err := n.GetInfo()
	log.Infow("sync node info", "info", info.JSON())
	if err == nil {
		if info.ID != m.cfg.Identity {
			m.local.Update(func(data *core.LocalData) {
				data.Nodes[info.ID] = info
			})
			m.connectRemoteDataStore(info.DataStore)
		}
	}
	if !n.IsClosed() {
		fmt.Println("node added:", n.ID())
		pushed = true
		m.Push(n)
	}
	wg := &sync.WaitGroup{}
	for !n.IsClosed() {
		wg.Add(1)
		m.syncPeers(wg, n)
		wg.Add(1)
		data, err := m.getLinkData(n)
		if err == nil {
			wg.Add(1)
			m.syncLDs(wg, n, data)
			wg.Add(1)
			m.syncInfo(wg, n, data)
		}

		//wait something done
		wg.Wait()
		time.Sleep(30 * time.Second)
	}
}

func (m *manager) getLinkData(node core.Node) ([]string, error) {
	ds, err := node.LDs()
	if err != nil {
		fmt.Println("failed to get link data", err)
		if err == ErrNoData {
			return []string{}, nil
		}
		//close connect when found err?
		return nil, err
	}
	return ds, nil
}

func (m *manager) connectRemoteDataStore(info core.DataStoreInfo) {
	timeout, cancelFunc := context.WithTimeout(context.TODO(), time.Second*30)
	defer cancelFunc()

	if m.addrCB != nil {
		addresses, err := basis.ParseAddresses(timeout, info.Addresses)
		if err != nil {
			return
		}
		total := len(addresses)
		for _, addr := range addresses {
			err = m.addrCB(addr)
			if err != nil {
				total = total - 1
				continue
			}
		}
		if total <= 0 {
			log.Infow("addr callback failed", "err", err, "addrinfo", info.Addresses)
		}
	}

}

func (m *manager) syncPeers(wg *sync.WaitGroup, n core.Node) {
	defer wg.Done()
	peers, err := n.Peers()
	if err != nil {
		return
	}
	for _, p := range peers {
		if len(p.Addrs) != 0 {
			_ = m.connectMultiAddr(p)
		}
	}
}

func decodeNode(m core.NodeManager, b []byte, api core.API) error {
	nodes := map[string]jsonNode{}
	err := json.Unmarshal(b, &nodes)
	if err != nil {
		return err
	}

	for _, nodes := range nodes {
		for _, addr := range nodes.Addrs {
			connectNode, err := ConnectNode(addr, 0, m.Local())
			if err != nil {
				continue
			}
			m.Push(connectNode)
			break
		}
	}
	return nil
}

func encodeNode(node core.Node) ([]byte, error) {
	n := map[string]jsonNode{
		node.ID(): {Addrs: node.Addrs()},
	}
	return json.Marshal(n)
}

// Close ...
func (m *manager) Close() {
	m.nodes.Close()
	m.hashNodes.Close()
}

// GetNode ...
func (m *manager) GetNode(id string) (n core.Node, b bool) {
	load, ok := m.connectNodes.Load(id)
	if ok {
		n, b = load.(core.Node)
		return
	}
	return
}

// AllNodes ...
func (m *manager) AllNodes() (map[string]core.Node, int, error) {
	nodes := make(map[string]core.Node)
	count := 0
	m.Range(func(key string, node core.Node) bool {
		nodes[key] = node
		node.Addrs()
		count++
		return true
	})
	return nodes, count, nil
}

// Conn ...
func (m *manager) Conn(c net.Conn) (core.Node, error) {
	return m.newConn(c)
}

func (m *manager) addNode(n core.Node) (bool, error) {
	if m.currentNodes.Load() > int32(m.cfg.Node.ConnectMax) {
		go m.nodeGC()
	}
	m.Push(n)
	return true, nil
}

func (m *manager) nodeGC() {
	if !m.gc.CAS(false, true) {
		return
	}
	defer m.gc.Store(false)
	m.connectNodes.Range(func(key, value interface{}) (deleted bool) {
		defer func() {
			keyStr, b := key.(string)
			if !b {
				return
			}
			if deleted {
				m.connectNodes.Delete(key)
				m.local.Update(func(data *core.LocalData) {
					delete(data.Nodes, keyStr)
				})
			}
		}()
		v, b := value.(core.Node)
		if !b {
			m.connectNodes.Delete(key)
			return true
		}
		ping, err := v.Ping()
		if err != nil {
			return true
		}
		if ping != "pong" {
			return true
		}

		return true
	})

}

func (m *manager) connectMultiAddr(info core.NodeInfo) error {
	if info.ID == m.cfg.Identity {
		return nil
	}
	log.Infow("info", "id", info.ID)
	_, ok := m.connectNodes.Load(info.ID)
	if ok {
		return nil
	}
	addrs := info.GetAddrs()
	if addrs == nil {
		return nil
	}
	for _, addr := range addrs {
		dialer := mnet.Dialer{
			Dialer: net.Dialer{
				Timeout: 3 * time.Second,
			},
		}
		dial, err := dialer.Dial(addr)
		if err != nil {
			fmt.Printf("link failed(%v)\n", err)
			return err
		}
		fmt.Printf("link success(%v)\n", addr)
		_, err = m.newConn(dial)
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("no link connect")
}

// RegisterAddrCallback ...
func (m *manager) RegisterAddrCallback(f func(info peer.AddrInfo) error) {
	m.addrCB = f
}

// ConnRemoteFromHash ...
func (m *manager) ConnRemoteFromHash(hash string) error {
	var nodes Nodes
	err := m.hashNodes.Load(hash, &nodes)
	if err != nil {
		return err
	}
	for s := range nodes.n {
		addr, err := DialFromStringAddr(s, 0)
		if err != nil {
			continue
		}
		_, err = m.Conn(addr)
		if err != nil {
			continue
		}
	}
	return nil
}

func (m *manager) syncInfo(wg *sync.WaitGroup, node core.Node, lds []string) {
	defer wg.Done()
	for _, ld := range lds {
		err := m.hashNodes.Update(ld, func(bytes []byte) (core.Marshaler, error) {
			nodes := NewNodes()
			err := nodes.Unmarshal(bytes)
			if err != nil {
				return nil, err
			}
			nodes.n[node.ID()] = true
			return nodes, nil
		})
		if err != nil {
			continue
		}
		fmt.Println("from:", node.ID(), "list:", ld)
	}
}

func (m *manager) syncLDs(wg *sync.WaitGroup, node core.Node, data []string) {
	defer wg.Done()
}
