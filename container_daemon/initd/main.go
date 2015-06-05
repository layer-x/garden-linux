package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"syscall"

	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/garden-linux/container_daemon"
	"github.com/cloudfoundry-incubator/garden-linux/container_daemon/unix_socket"
	"github.com/cloudfoundry-incubator/garden-linux/containerizer"
	"github.com/cloudfoundry-incubator/garden-linux/containerizer/system"
)

func main() {
	logFile, _ := ioutil.TempFile("", "initd.log")
	os.Stdout = logFile
	os.Stderr = logFile
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "initd: panicked: %s\n", r)
			os.Exit(4)
		}
	}()

	cf_lager.AddFlags(flag.CommandLine)
	flag.Parse()
	logger, _ := cf_lager.New("init")

	syncWriter := os.NewFile(uintptr(4), "/dev/sync_writer")
	syscall.RawSyscall(syscall.SYS_FCNTL, uintptr(4), syscall.F_SETFD, syscall.FD_CLOEXEC)
	defer syncWriter.Close()

	sync := &containerizer.PipeSynchronizer{
		Writer: syncWriter,
	}

	reaper := system.StartReaper(logger)
	defer reaper.Stop()

	daemon := &container_daemon.ContainerDaemon{
		CmdPreparer: &container_daemon.ProcessSpecPreparer{
			Users:           container_daemon.LibContainerUser{},
			Rlimits:         &container_daemon.RlimitsManager{},
			ProcStarterPath: "/sbin/proc_starter",
		},
		Spawner: &container_daemon.Spawn{
			Runner: reaper,
			PTY:    system.KrPty,
		},
	}

	socketFile := os.NewFile(uintptr(5), "/dev/host.sock")
	syscall.RawSyscall(syscall.SYS_FCNTL, uintptr(5), syscall.F_SETFD, syscall.FD_CLOEXEC)
	defer socketFile.Close()

	listener, err := unix_socket.NewListenerFromFile(socketFile)
	if err != nil {
		fail(fmt.Sprintf("initd: failed to create listener: %s\n", err), 5)
	}

	if err := sync.SignalSuccess(); err != nil {
		fail(fmt.Sprintf("signal host: %s", err), 6)
	}

	if err := daemon.Run(listener); err != nil {
		fail(fmt.Sprintf("run daemon: %s", err), 7)
	}
}

func fail(err string, code int) {
	fmt.Fprintf(os.Stderr, "initd: %s\n", err)
	os.Exit(code)
}
