// +build linux
package net_fence_test

import (
	"bufio"
	"fmt"
	"github.com/onsi/ginkgo/config"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/milosgajdos83/tenus"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	verbose            bool
	netFenceBin        string
	containerInitBin   string
	inContainerTestBin string
	ctr                *container
)

func buildInContainerTest() string {
	cmd := exec.Command("ginkgo", "build", "-race", "_hidden/in_container")
	out, err := cmd.Output()
	Ω(out).Should(ContainSubstring(" compiled "))
	Ω(err).ShouldNot(HaveOccurred())

	tmpDir, err := ioutil.TempDir("", "garden-test")
	Ω(err).ShouldNot(HaveOccurred())

	testBin := filepath.Join(tmpDir, "in_container.test")
	Ω(os.Rename("./_hidden/in_container/in_container.test", testBin)).ShouldNot(HaveOccurred())
	return testBin
}

var _ = Describe("Configure", func() {

	BeforeEach(func() {
		verbose = config.DefaultReporterConfig.Verbose

		netFencePath, err := gexec.Build("github.com/cloudfoundry-incubator/garden-linux/fences/mains/net-fence", "-race")
		Ω(err).ShouldNot(HaveOccurred())
		netFenceBin = string(netFencePath)

		containerInitPath, err := gexec.Build("github.com/cloudfoundry-incubator/garden-linux/integration/fences/net_fence/container-init", "-race")
		Ω(err).ShouldNot(HaveOccurred())
		containerInitBin = string(containerInitPath)

		inContainerTestBin = buildInContainerTest()

		_, err = tenus.NewVethPairWithOptions("testHostIfcName", tenus.VethOptions{
			PeerName:   "testPeerIfcName",
			TxQueueLen: 1,
		})
		Ω(err).ShouldNot(HaveOccurred())

		ctr, err = createContainer(syscall.CLONE_NEWNET, inContainerTestBin)
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		err := tenus.DeleteLink("testHostIfcName")
		Ω(err).ShouldNot(HaveOccurred())

		if ctrProc, err := os.FindProcess(ctr.cmd.Process.Pid); err == nil {
			ctrProc.Kill()
		}
	})

	It("configures a network interface in the global network namespace", func() {

		configureHost("testHostIfcName", "10.2.3.2/30", "testPeerIfcName", ctr.cmd.Process.Pid)

		if verbose {
			fmt.Println("\nGinkgo inContainer tests:\n<<----")
		}

		ctr.proceed()

		Ω(ctr.wait()).ShouldNot(HaveOccurred())

		if verbose {
			fmt.Println("\n---->>\nGinkgo inContainer tests ended.")
		}

	})
})

func configureHost(hostIfc, hostSubnet, ctrIfc string, pid int) {
	// Move the container's ethernet interface into the network namespace.
	moveInterfaceToNamespace(ctrIfc, pid)

	// Add the host address
	addIpAddress(hostSubnet, hostIfc)

	// Bring the host's ethernet interface up
	ipLinkUp(hostIfc)
}

func moveInterfaceToNamespace(ifc string, pid int) {
	cmd := exec.Command("ip", "link", "set", ifc, "netns", fmt.Sprintf("%d", pid))
	err := cmd.Run()
	Ω(err).ShouldNot(HaveOccurred())
}

func addIpAddress(hostSubnet, ifc string) {
	cmd := exec.Command("ip", "address", "add", hostSubnet, "dev", ifc)
	err := cmd.Run()
	Ω(err).ShouldNot(HaveOccurred())
}

func ipLinkUp(ifc string) {
	cmd := exec.Command("ip", "link", "set", ifc, "up")
	err := cmd.Run()
	Ω(err).ShouldNot(HaveOccurred())
}

type container struct {
	rendezvousChan chan string
	outputChan     chan interface{}
	cmd            *exec.Cmd
	fd             net.Conn
}

// Creates a collection of namespaces defined by cloneFlags and starts an init process.
// When the init process has reached a rendezvous point, returns.
func createContainer(cloneFlags int, executable string, args ...string) (*container, error) {
	err := checkRoot()
	if err != nil {
		return nil, err
	}

	initArgs := make([]string, 0, len(args)+1)
	initArgs = append(initArgs, executable)
	initArgs = append(initArgs, args...)

	cmd := exec.Command(containerInitBin, initArgs...)

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Cloneflags = uintptr(cloneFlags)
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL

	container := &container{
		rendezvousChan: make(chan string),
		outputChan:     make(chan interface{}),
	}

	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	go waitForEof(container.outputChan, stdOut)

	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	go waitForEof(container.outputChan, stdErr)

	go listenForClient(container)

	container.cmd = cmd
	err = cmd.Start()
	if err != nil {
		log.Fatal("Start failed:", err)
	}

	// wait for child to reach rendezvous point.
	err = container.rendezvous()
	if err != nil {
		log.Fatal("rendezvous failed:", err)
	}

	return container, nil
}

func waitForEof(ch chan<- interface{}, reader io.Reader) {
	defer func() {
		ch <- nil
	}()
	data := make([]byte, 1024)
	for {
		n, err := reader.Read(data)
		if verbose && n > 0 {
			fmt.Println(string(data[:n]))
		}
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Error reading from reader %v: %s\n", reader, err)
			}
			return
		}
	}
}

// Waits for output from the client and send the output on the rendezvous channel.
// Sets the container connection.
func listenForClient(ctr *container) {
	l, err := net.Listen("unix", "/tmp/test-rendezvous.sock")
	if err != nil {
		log.Fatal("listen error:", err)
	}
	ctr.fd, err = l.Accept()
	if err != nil {
		log.Fatal("accept error:", err)
	}
	lineReader := bufio.NewReader(ctr.fd)
	str, err := lineReader.ReadString('\n')
	if err != nil {
		log.Fatal("ReadString error:", err)
	}
	ctr.rendezvousChan <- str
}

func (c *container) rendezvous() error {
	str := <-c.rendezvousChan
	if str != "rendezvous\n" {
		log.Fatal("unexpected rendezvous string from client")
	}
	return nil
}

// Allows the container to proceed from the rendezbous point to run the executable to completion.
// Pre-condition: the container must be at the rendezvous point.
func (c *container) proceed() error {
	// let the child continue
	c.fd.Write([]byte("rendezvous\n"))
	return nil
}

func (c *container) wait() error {
	<-c.outputChan
	<-c.outputChan
	return c.cmd.Wait()
}

func checkRoot() error {
	if uid := os.Getuid(); uid != 0 {
		return fmt.Errorf("createContainer must be run as root. Getuid returned %d", uid)
	}
	return nil
}
