package main

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/pivotal-golang/lager"
)

func (s *YarnBackend) HandleBindMounts(c garden.ContainerSpec, containerDir string) error {

	for _, b := range c.BindMounts {
		var flags uintptr = syscall.MS_BIND

		if b.Mode == garden.BindMountModeRO {
			flags = flags | syscall.MS_RDONLY
		}

		src := b.SrcPath
		if b.Origin == garden.BindMountOriginContainer {
			src = filepath.Join(containerDir, src)
		}
		dst := filepath.Join(containerDir, b.DstPath)
		s.Logger.Debug("Trying to mount", lager.Data{"src": src, "dst": dst})
		// create dst if not exists
		os.MkdirAll(dst, 0755)

		err := syscall.Mount(src, dst, "", flags, "")
		if err != nil {
			s.Logger.Debug("MOUNT ERROR", lager.Data{"src": src, "dst": dst, "err": err})
			return err
		}
	}

	return nil
}
