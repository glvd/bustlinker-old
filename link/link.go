package link

import (
	"fmt"

	"github.com/glvd/bustlinker/core"
)

type Linker interface {
	Start() error
}

type link struct {
	node *core.IpfsNode
}

func (l link) Start() error {
	fmt.Println("Link start")
	return nil
}

func New(node *core.IpfsNode) (Linker, error) {
	return &link{node: node}, nil
}

var _ Linker = &link{}
