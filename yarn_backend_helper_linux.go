package main

import (
	"path/filepath"
	"syscall"

	"github.com/cloudfoundry-incubator/garden"
)

func HandleBindMounts(c garden.ContainerSpec, containerDir string) {

	for _, b := range c.BindMounts {
		var flags uintptr = syscall.MS_BIND

		if b.Mode == garden.BindMountModeRO {
			flags = flags | syscall.MS_RDONLY
		}

		src := b.SrcPath
		if b.Origin == garden.BindMountOriginContainer {
			src = filepath.Join(containerDir, src)
		}
		syscall.Mount(src, filepath.Join(containerDir, b.DstPath), "", flags, 0)
	}
}
