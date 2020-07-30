// +build linux darwin freebsd netbsd openbsd
// +build !nofuse

package ipns

import (
	core "github.com/glvd/starship/core"
	coreapi "github.com/glvd/starship/core/coreapi"
	mount "github.com/glvd/starship/fuse/mount"
)

// Mount mounts ipns at a given location, and returns a mount.Mount instance.
func Mount(ipfs *core.IpfsNode, ipnsmp, ipfsmp string) (mount.Mount, error) {
	coreApi, err := coreapi.NewCoreAPI(ipfs)
	if err != nil {
		return nil, err
	}

	cfg, err := ipfs.Repo.Config()
	if err != nil {
		return nil, err
	}

	allow_other := cfg.Mounts.FuseAllowOther

	fsys, err := NewFileSystem(ipfs.Context(), coreApi, ipfsmp, ipnsmp)
	if err != nil {
		return nil, err
	}

	return mount.NewMount(ipfs.Process, fsys, ipnsmp, allow_other)
}