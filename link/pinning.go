package link

import (
	"context"
	"github.com/glvd/bustlinker/core"
	"github.com/glvd/bustlinker/core/coreapi"
	"go.uber.org/atomic"
	"sync"
)

type Pinning interface {
	Get() []string
	Clear()
	AddSync(pin string)
	Set(pins []string)
}

type pinning struct {
	ctx      context.Context
	cancel   context.CancelFunc
	running  *atomic.Bool
	syncing  *sync.Pool
	node     *core.IpfsNode
	pins     map[string]bool
	pinsLock *sync.RWMutex
}

func (p *pinning) Get() []string {
	var pins []string
	p.pinsLock.RLock()
	if len(p.pins) == 0 {
		return pins
	}
	for pin := range p.pins {
		pins = append(pins, pin)
	}
	p.pinsLock.RUnlock()
	return pins
}

func (p *pinning) Clear() {
	p.pinsLock.Lock()
	p.pins = make(map[string]bool)
	p.pinsLock.Unlock()
}

func (p *pinning) AddSync(pin string) {
	p.syncing.Put(pin)
}

func (p *pinning) Add(pin string) {
	p.pinsLock.Lock()
	p.pins[pin] = true
	p.pinsLock.Unlock()
}

func (p *pinning) Set(pins []string) {
	ps := make(map[string]bool, len(pins))
	for _, pin := range pins {
		ps[pin] = true
	}
	p.pinsLock.Lock()
	p.pins = ps
	p.pinsLock.Unlock()
}

func (p *pinning) Pause() {
	p.running.Store(false)
}

func (p *pinning) Resume() {
	if p.running.CAS(false, true) {
		go p.run()
	}
}

func (p *pinning) run() {
	p.ctx, p.cancel = context.WithCancel(context.TODO())
	api, err := coreapi.NewCoreAPI(p.node)
	if err != nil {
		log.Error("run pinning failed:", err)
		return
	}
	for p.running.Load() {
		api.Pin()
	}
}

func newPinning(node *core.IpfsNode) Pinning {
	p := &pinning{
		running:  atomic.NewBool(true),
		node:     node,
		pins:     make(map[string]bool),
		pinsLock: &sync.RWMutex{},
	}
	go p.run()
	return p
}
