package link

import (
	"github.com/glvd/bustlinker/core"
	"sync"
)

type Pinning interface {
	Get() []string
	Clear()
	Add(pin string)
	Set(pins []string)
}

type pinning struct {
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

func newPinning(node *core.IpfsNode) Pinning {
	return &pinning{
		node:     node,
		pins:     make(map[string]bool),
		pinsLock: &sync.RWMutex{},
	}
}
