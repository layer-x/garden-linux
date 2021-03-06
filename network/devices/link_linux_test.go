package devices_test

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cloudfoundry-incubator/garden-linux/network/devices"
	"github.com/docker/libcontainer/netlink"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Link Management", func() {
	var (
		l    devices.Link
		name string
		intf *net.Interface
	)

	BeforeEach(func() {
		cmd, err := gexec.Start(exec.Command("sh", "-c", "mountpoint /sys || mount -t sysfs sysfs /sys"), GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(cmd).Should(gexec.Exit(0))

		name = fmt.Sprintf("gdn-test-%d", GinkgoParallelNode())
		Expect(netlink.NetworkLinkAdd(name, "dummy")).To(Succeed())
		intf, _ = net.InterfaceByName(name)
	})

	AfterEach(func() {
		cleanup(name)
	})

	Describe("AddIP", func() {
		Context("when the interface exists", func() {
			It("adds the IP succesffuly", func() {
				ip, subnet, _ := net.ParseCIDR("1.2.3.4/5")
				Expect(l.AddIP(intf, ip, subnet)).To(Succeed())

				intf, err := net.InterfaceByName(name)
				Expect(err).ToNot(HaveOccurred())
				addrs, err := intf.Addrs()
				Expect(err).ToNot(HaveOccurred())

				Expect(addrs).To(HaveLen(1))
				Expect(addrs[0].String()).To(Equal("1.2.3.4/5"))
			})
		})
	})

	Describe("SetUp", func() {
		Context("when the interface does not exist", func() {
			It("returns an error", func() {
				Expect(l.SetUp(&net.Interface{Name: "something"})).ToNot(Succeed())
			})
		})

		Context("when the interface exists", func() {
			Context("and it is down", func() {
				It("should bring the interface up", func() {
					Expect(l.SetUp(intf)).To(Succeed())

					intf, err := net.InterfaceByName(name)
					Expect(err).ToNot(HaveOccurred())
					Expect(intf.Flags & net.FlagUp).To(Equal(net.FlagUp))
				})
			})

			Context("and it is already up", func() {
				It("should still succeed", func() {
					Expect(l.SetUp(intf)).To(Succeed())
					Expect(l.SetUp(intf)).To(Succeed())

					intf, err := net.InterfaceByName(name)
					Expect(err).ToNot(HaveOccurred())
					Expect(intf.Flags & net.FlagUp).To(Equal(net.FlagUp))
				})
			})
		})
	})

	Describe("SetMTU", func() {
		Context("when the interface does not exist", func() {
			It("returns an error", func() {
				Expect(l.SetMTU(&net.Interface{Name: "something"}, 1234)).ToNot(Succeed())
			})
		})

		Context("when the interface exists", func() {
			It("sets the mtu", func() {
				Expect(l.SetMTU(intf, 1234)).To(Succeed())

				intf, err := net.InterfaceByName(name)
				Expect(err).ToNot(HaveOccurred())
				Expect(intf.MTU).To(Equal(1234))
			})
		})
	})

	Describe("SetNs", func() {
		BeforeEach(func() {
			cmd, err := gexec.Start(exec.Command("sh", "-c", "ip netns add gdnsetnstest"), GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(cmd).Should(gexec.Exit(0))
		})

		AfterEach(func() {
			cmd, err := gexec.Start(exec.Command("sh", "-c", "ip netns delete gdnsetnstest"), GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(cmd).Should(gexec.Exit(0))
		})

		It("moves the interface in to the given namespace by pid", func() {
			// look at this perfectly ordinary hat
			netns, err := gexec.Start(exec.Command("ip", "netns", "exec", "gdnsetnstest", "sleep", "6312736"), GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			defer netns.Kill()

			// (it has the following pid)
			ps, err := gexec.Start(exec.Command("sh", "-c", "ps -A -opid,command | grep 'sleep 6312736' | head -n 1 | awk '{print $1}'"), GinkgoWriter, GinkgoWriter) // look at my hat
			Expect(err).ToNot(HaveOccurred())
			Eventually(ps).Should(gexec.Exit(0))
			pid, err := strconv.Atoi(strings.TrimSuffix(string(ps.Out.Contents()), "\n"))
			Expect(err).ToNot(HaveOccurred())

			// I wave the magic wand
			Expect(l.SetNs(intf, pid)).To(Succeed())

			// the bunny has vanished! where is the bunny?
			intfs, _ := net.Interfaces()
			Expect(intfs).ToNot(ContainElement(intf))

			// oh my word it's in the hat!
			session, err := gexec.Start(exec.Command("sh", "-c", fmt.Sprintf("ip netns exec gdnsetnstest ifconfig %s", name)), GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

		})
	})

	Describe("InterfaceByName", func() {
		Context("when the interface exists", func() {
			It("returns the interface with the given name, and true", func() {
				returnedIntf, found, err := l.InterfaceByName(name)
				Expect(err).ToNot(HaveOccurred())

				Expect(returnedIntf).To(Equal(intf))
				Expect(found).To(BeTrue())
			})
		})

		Context("when the interface does not exist", func() {
			It("does not return an error", func() {
				_, found, err := l.InterfaceByName("sandwich")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("List", func() {
		It("lists all the interfaces", func() {
			names, err := l.List()
			Expect(err).ToNot(HaveOccurred())

			Expect(names).To(ContainElement(name))
		})
	})

	Describe("Statistics", func() {

		Context("When the interface exist", func() {
			BeforeEach(func() {
				cmd, err := gexec.Start(exec.Command(
					"sh", "-c", `
					ip netns add netns1
					ip link add veth0 type veth peer name veth1
					ip link set veth1 netns netns1
					ip netns exec netns1 ifconfig veth1 10.1.1.1/24 up
					ifconfig veth0 10.1.1.2/24 up
					`,
				), GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(cmd, "10s").Should(gexec.Exit(0))
			})

			AfterEach(func() {
				cmd, err := gexec.Start(exec.Command(
					"sh", "-c", `
					ip netns exec netns1 ip link del veth1
					ip netns delete netns1
					`,
				), GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(cmd).Should(gexec.Exit(0))
			})

			It("Gets statistics from the interface", func() {
				link := devices.Link{Name: "veth0"}
				beforeStat, err := link.Statistics()
				Expect(err).ToNot(HaveOccurred())
				cmd, err := gexec.Start(exec.Command(
					"sh", "-c", `
					ping -c 10 -s 80 10.1.1.1
					`,
				), GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(cmd, "15s").Should(gexec.Exit(0))

				afterStat, err := link.Statistics()
				Expect(err).ToNot(HaveOccurred())

				// size of ping packet is 42 + payload_size (80 bytes)
				// there could be additional arp messages transferred and recieved
				// so check for range instead of absolute values
				Expect(afterStat.TxBytes).To(BeNumerically(">=", beforeStat.TxBytes+(10*(42+80))))
				Expect(afterStat.TxBytes).To(BeNumerically("<", beforeStat.TxBytes+(10*(42+80))+1000))
				Expect(afterStat.RxBytes).To(BeNumerically(">=", beforeStat.RxBytes+(10*(42+80))))
				Expect(afterStat.RxBytes).To(BeNumerically("<", beforeStat.RxBytes+(10*(42+80))+1000))
			})
		})

		Context("when the interface does not exist", func() {
			It("Gets statistics return an error", func() {
				link := devices.Link{Name: "non-existent-intf"}
				_, err := link.Statistics()
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
