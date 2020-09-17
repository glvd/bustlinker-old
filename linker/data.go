package linker

import (
	"encoding/json"
	"github.com/libp2p/go-libp2p-core/peer"
	"sync"
	"time"
)

// SafeLocalData ...
type SafeLocalData interface {
	JSON() string
	Update(f func(data *Info))
	Data() (data Info)
}

// SafeLocalData ...
type safeLocalData struct {
	lock sync.RWMutex
	data Info
}

// Info ...
type Info struct {
	Initialized bool
	Node        peer.AddrInfo
	Nodes       map[string]peer.AddrInfo //readonly or change by update
	LDs         map[string]uint8         //readonly or change by update:ipfs linked data
	Addrs       []string
	LastUpdate  int64
}

// DefaultLocalData ...
func DefaultLocalData() *Info {
	return &Info{
		LDs:        make(map[string]uint8),
		LastUpdate: time.Now().Unix(),
		Nodes:      make(map[string]peer.AddrInfo),
	}
}

// Marshal ...
func (l *safeLocalData) Marshal() ([]byte, error) {
	l.lock.RLock()
	marshal, err := json.Marshal(l.data)
	l.lock.RUnlock()
	if err != nil {
		return nil, err
	}
	return marshal, err
}

// Unmarshal ...
func (l *safeLocalData) Unmarshal(bytes []byte) (err error) {
	l.lock.Lock()
	err = json.Unmarshal(bytes, &l.data)
	l.lock.Unlock()
	return
}

// JSON ...
func (l *safeLocalData) JSON() string {
	marshal, err := l.Marshal()
	if err != nil {
		return ""
	}
	return string(marshal)
}

// Update ...
func (l *safeLocalData) Update(f func(data *Info)) {
	l.lock.Lock()
	f(&l.data)
	l.lock.Unlock()
}

// Data ...
func (l *safeLocalData) Data() (data Info) {
	//data = LinkInfo{
	//	Initialized: false,
	//	Node:        NodeInfo{},
	//	Nodes:       make(map[string]NodeInfo),
	//	LDs:         make(map[string]uint8),
	//	Addrs:       nil,
	//	LastUpdate:  0,
	//}
	l.lock.Lock()
	//data.Node = l.data.Node
	//if l.data.Addrs != nil {
	//	copy(data.Addrs, l.data.Addrs)
	//}
	//for s := range l.data.Nodes {
	//	data.Nodes[s] = l.data.Nodes[s]
	//}
	//for s := range l.data.LDs {
	//	data.LDs[s] = l.data.LDs[s]
	//}
	data = l.data
	l.lock.Unlock()
	return
}

// Safe ...
func (l Info) Safe() SafeLocalData {
	return &safeLocalData{
		lock: sync.RWMutex{},
		data: l,
	}
}
