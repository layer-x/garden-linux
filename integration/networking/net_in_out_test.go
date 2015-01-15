package networking_test

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	archiver "github.com/pivotal-golang/archiver/extractor/test_helper"
)

var _ = Describe("Net In/Out", func() {
	var (
		container      garden.Container
		otherContainer garden.Container

		containerNetwork string
		denyRange        string
		allowRange       string
	)

	BeforeEach(func() {
		denyRange = ""
		allowRange = ""
	})

	JustBeforeEach(func() {
		client = startGarden(
			"-denyNetworks", strings.Join([]string{
				denyRange,
				allowRange, // so that it can be overridden by allowNetworks below
			}, ","),
			"-allowNetworks", allowRange,
		)

		var err error
		container, err = client.Create(garden.ContainerSpec{Network: containerNetwork, Privileged: true})
		Ω(err).ShouldNot(HaveOccurred())

		Ω(container.StreamIn("bin/", tgzReader(netdogBin))).Should(Succeed())
	})

	AfterEach(func() {
		err := client.Destroy(container.Handle())
		Ω(err).ShouldNot(HaveOccurred())
	})

	runInContainer := func(container garden.Container, script string) (garden.Process, *gbytes.Buffer) {
		out := gbytes.NewBuffer()
		process, err := container.Run(garden.ProcessSpec{
			Path: "sh",
			Args: []string{"-c", script},
		}, garden.ProcessIO{
			Stdout: io.MultiWriter(out, GinkgoWriter),
			Stderr: GinkgoWriter,
		})
		Ω(err).ShouldNot(HaveOccurred())

		return process, out
	}

	Context("external addresses", func() {
		var (
			ByAllowingTCP, ByAllowingICMP, ByRejectingTCP, ByRejectingICMP func()
		)

		BeforeEach(func() {
			ByAllowingTCP = func() {
				By("allowing outbound tcp traffic", func() {
					process, _ := runInContainer(
						container,
						fmt.Sprintf("(echo 'GET / HTTP/1.1'; echo 'Host: example.com'; echo) | nc -w5 %s 80", externalIP),
					)

					Ω(process.Wait()).Should(Equal(0))
				})
			}

			ByAllowingICMP = func() {
				if err := exec.Command("sh", "-c", fmt.Sprintf("ping -c 1 -w 1 %s", externalIP)).Run(); err != nil {
					fmt.Println("Ginkgo host environment cannot ping out, skipping ICMP out test: ", err)
					return
				}

				By("allowing outbound icmp traffic", func() {
					// sacrificial ping, which appears not to work on first packet
					runInContainer(
						container,
						fmt.Sprintf("ping -c 1 %s", externalIP),
					)
					_, out := runInContainer(
						container,
						fmt.Sprintf("ping -c 1 -w 1 %s", externalIP),
					)

					Eventually(out, "5s").Should(gbytes.Say(" 0% packet loss"), "lost packets on ping")
				})
			}

			ByRejectingTCP = func() {
				By("rejecting outbound tcp traffic", func() {
					process, _ := runInContainer(
						container,
						fmt.Sprintf("(echo 'GET / HTTP/1.1'; echo 'Host: example.com'; echo) | nc -w5 %s 80", externalIP),
					)

					Ω(process.Wait()).Should(Equal(1))
				})
			}

			ByRejectingICMP = func() {
				if err := exec.Command("sh", "-c", fmt.Sprintf("ping  -c 1 -w 1 %s", externalIP)).Run(); err != nil {
					fmt.Println("Ginkgo host environment cannot ping out, skipping ICMP out test: ", err)
					return
				}

				By("rejecting outbound icmp traffic", func() {
					_, out := runInContainer(
						container,
						fmt.Sprintf("ping  -c 1 -w 1 %s", externalIP),
					)

					Eventually(out, "5s").Should(gbytes.Say(" 100% packet loss"))
				})
			}
		})

		Context("when the target address is inside DENY_NETWORKS", func() {
			BeforeEach(func() {
				denyRange = "0.0.0.0/0"
				allowRange = "9.9.9.9/30"
				containerNetwork = fmt.Sprintf("10.1%d.0.0/24", GinkgoParallelNode())
			})

			Context("by default", func() {
				It("disallows connections", func() {
					ByRejectingTCP()
					ByRejectingICMP()
				})
			})

			Context("after a net_out of another range", func() {
				It("does not allow connections to that address", func() {
					container.NetOut("1.2.3.4/30", 0, "", garden.ProtocolAll, -1, -1)
					ByRejectingTCP()
					ByRejectingICMP()
				})
			})

			Context("after net_out allows tcp traffic to that IP and port", func() {
				Context("when no port is specified", func() {
					It("allows both tcp and icmp to that address", func() {
						err := container.NetOut(externalIP.String(), 0, "", garden.ProtocolAll, -1, -1)
						Ω(err).ShouldNot(HaveOccurred())
						ByAllowingTCP()
						ByAllowingICMP()
					})
				})
			})

			Context("after net_out allows tcp traffic to a range of IP addresses", func() {
				It("allows tcp to an address in the range", func() {
					err := container.NetOut(externalIP.String()+"-"+"255.255.255.254", 0, "", garden.ProtocolTCP, -1, -1)
					Ω(err).ShouldNot(HaveOccurred())
					ByAllowingTCP()
				})
			})

			Describe("allowing individual protocols", func() {
				// To prevent test pollution due to connection tracking, each test
				// should use a distinct container IP address.
				Context("when all TCP traffic is allowed", func() {
					BeforeEach(func() {
						containerNetwork = fmt.Sprintf("10.1%d.2.0/24", GinkgoParallelNode())
					})

					It("allows TCP and blocks ICMP", func() {
						err := container.NetOut(externalIP.String(), 0, "", garden.ProtocolTCP, -1, -1)
						Ω(err).ShouldNot(HaveOccurred())
						ByAllowingTCP()
						ByRejectingICMP()
					})
				})

				Context("when ICMP non-ping type traffic is allowed", func() {
					BeforeEach(func() {
						containerNetwork = fmt.Sprintf("10.1%d.3.0/24", GinkgoParallelNode())
					})

					It("allows ICMP pings and blocks TCP", func() {
						err := container.NetOut(externalIP.String(), 0, "", garden.ProtocolICMP, 13, 0)
						Ω(err).ShouldNot(HaveOccurred())
						ByRejectingICMP()
						ByRejectingTCP()
					})
				})

				Context("when ICMP ping type/code traffic is allowed", func() {
					BeforeEach(func() {
						containerNetwork = fmt.Sprintf("10.1%d.4.0/24", GinkgoParallelNode())
					})

					It("allows ICMP pings and blocks TCP", func() {
						err := container.NetOut(externalIP.String(), 0, "", garden.ProtocolICMP, 8, 4)
						ByRejectingICMP()
						ByRejectingTCP()
						err = container.NetOut(externalIP.String(), 0, "", garden.ProtocolICMP, 8, 0)
						Ω(err).ShouldNot(HaveOccurred())
						ByAllowingICMP()
						ByRejectingTCP()
					})
				})

				Context("when all ICMP traffic is allowed", func() {
					BeforeEach(func() {
						containerNetwork = fmt.Sprintf("10.1%d.5.0/24", GinkgoParallelNode())
					})

					It("allows ICMP and blocks TCP", func() {
						err := container.NetOut(externalIP.String(), 0, "", garden.ProtocolICMP, -1, -1)
						Ω(err).ShouldNot(HaveOccurred())
						ByAllowingICMP()
						ByRejectingTCP()
					})
				})
			})
		})

		Context("when the target address is inside ALLOW_NETWORKS", func() {
			BeforeEach(func() {
				denyRange = "0.0.0.0/0"
				allowRange = "0.0.0.0/0"
				containerNetwork = fmt.Sprintf("10.1%d.0.0/24", GinkgoParallelNode())
			})

			It("allows connections", func() {
				ByAllowingTCP()
				ByAllowingICMP()
			})
		})

		Context("when the target address is in neither ALLOW_NETWORKS nor DENY_NETWORKS", func() {
			BeforeEach(func() {
				denyRange = "4.4.4.4/30"
				allowRange = "4.4.4.4/30"
				containerNetwork = fmt.Sprintf("10.1%d.0.0/24", GinkgoParallelNode())
			})

			It("allows connections", func() {
				ByAllowingTCP()
				ByAllowingICMP()
			})
		})
	})

	Describe("Other Containers", func() {
		var (
			udpListenerOut *gbytes.Buffer
		)

		const tcpPort = 8080
		const udpPort = 8081
		const tcpPortRange = "8080:8090"
		const udpPortRange = "8081:8091"

		targetIP := func(c garden.Container) string {
			info, err := c.Info()
			Ω(err).ShouldNot(HaveOccurred())
			return info.ContainerIP
		}

		ByAllowingTCP := func() {
			By("allowing tcp traffic to it", func() {
				process, _ := runInContainer(
					container,
					fmt.Sprintf("echo hello | nc -w 1 %s %d", targetIP(otherContainer), tcpPort),
				)

				Ω(process.Wait()).Should(Equal(0))
			})
		}

		ByAllowingUDP := func() {
			By("allowing udp traffic to it", func() {
				process, _ := runInContainer(
					container,
					fmt.Sprintf("echo ok | ~/bin/netdog send %s:%d", targetIP(otherContainer), udpPort),
				)

				Ω(process.Wait()).Should(Equal(0))
				Eventually(udpListenerOut).Should(gbytes.Say("ok"))
			})
		}

		ByAllowingICMP := func() {
			By("allowing icmp traffic to it", func() {
				_, out := runInContainer(
					container,
					fmt.Sprintf("ping  -c 1 -w 1 %s", targetIP(otherContainer)),
				)

				Eventually(out, "5s").Should(gbytes.Say(" 0% packet loss"))
			})
		}

		ByRejectingTCP := func() {
			By("not allowing tcp traffic to it", func() {
				process, _ := runInContainer(
					container,
					fmt.Sprintf("echo hello | nc -w 4 %s %d", targetIP(otherContainer), tcpPort),
				)

				Ω(process.Wait()).Should(Equal(1))
			})
		}

		ByRejectingUDP := func() {
			By("not allowing udp traffic to it", func() {
				process, _ := runInContainer(
					container,
					fmt.Sprintf("echo ok | ~/bin/netdog send %s:%d", targetIP(otherContainer), udpPort),
				)

				Ω(process.Wait()).Should(Equal(0)) // udp is connectionless, we can send, it just shouldn't be received
				Consistently(udpListenerOut, "2s", "500ms").ShouldNot(gbytes.Say("ok"))
			})
		}

		ByRejectingICMP := func() {
			By("not allowing icmp traffic to it", func() {
				_, out := runInContainer(
					container,
					fmt.Sprintf("ping -c 1 -w 1 %s", targetIP(otherContainer)),
				)

				Eventually(out, "5s").Should(gbytes.Say(" 100% packet loss"))
			})
		}

		Context("containers in the same subnet", func() {
			JustBeforeEach(func() {
				var err error
				otherContainer, err = client.Create(garden.ContainerSpec{Network: containerNetwork})
				Ω(err).ShouldNot(HaveOccurred())

				runInContainer(otherContainer, fmt.Sprintf("echo hello | nc -l -p %d", tcpPort)) //tcp

				Ω(otherContainer.StreamIn("./bin/", tgzReader(netdogBin))).Should(Succeed())
				_, udpListenerOut = runInContainer(otherContainer, fmt.Sprintf("echo hello | ~/bin/netdog listen %d", udpPort)) //udp
				Eventually(udpListenerOut).Should(gbytes.Say("listening"))
			})

			Context("even if the address is in deny networks", func() {
				BeforeEach(func() {
					denyRange = "0.0.0.0/8"
					allowRange = ""
					containerNetwork = fmt.Sprintf("10.1%d.0.0/24", GinkgoParallelNode())
				})

				It("allows connections", func() {
					ByAllowingICMP()
					ByAllowingTCP()
					ByAllowingUDP()
				})
			})
		})

		Context("containers in other subnets", func() {
			var (
				otherContainerNetwork *net.IPNet
			)

			BeforeEach(func() {
				_, otherContainerNetwork, _ = net.ParseCIDR(fmt.Sprintf("10.2%d.0.1/24", GinkgoParallelNode()))
			})

			JustBeforeEach(func() {
				var err error
				otherContainer, err = client.Create(garden.ContainerSpec{Network: otherContainerNetwork.String()})
				Ω(err).ShouldNot(HaveOccurred())
				runInContainer(otherContainer, fmt.Sprintf("echo hello | nc -l -p %d", tcpPort)) //tcp

				Ω(otherContainer.StreamIn("bin/", tgzReader(netdogBin))).Should(Succeed())
				_, udpListenerOut = runInContainer(otherContainer, fmt.Sprintf("echo hello | ~/bin/netdog listen %d", udpPort)) //udp
				Eventually(udpListenerOut).Should(gbytes.Say("listening"))
			})

			Context("when the target address is inside DENY_NETWORKS", func() {
				BeforeEach(func() {
					denyRange = "10.0.0.0/8"
					allowRange = ""
					containerNetwork = fmt.Sprintf("10.1%d.0.0/24", GinkgoParallelNode())
				})

				Context("by default", func() {
					It("does not allow connections", func() {
						ByRejectingICMP()
						ByRejectingUDP()
						ByRejectingTCP()
					})
				})

				Context("after a net_out of another range", func() {
					It("still does not allow connections to that address", func() {
						container.NetOut("1.2.3.4/30", 0, "", garden.ProtocolAll, -1, -1)
						ByRejectingICMP()
						ByRejectingUDP()
						ByRejectingTCP()
					})
				})

				Context("after net_out allows all traffic to that IP and port", func() {
					Context("when no port is specified", func() {
						It("allows both tcp and icmp to that address", func() {
							container.NetOut(otherContainerNetwork.String(), 0, "", garden.ProtocolAll, -1, -1)
							ByAllowingICMP()
							ByAllowingUDP()
							ByAllowingTCP()
						})
					})

					Context("when a port is specified", func() {
						It("allows tcp connections to that port", func() {
							container.NetOut(otherContainerNetwork.String(), 12345, "", garden.ProtocolTCP, -1, -1) // wrong port
							ByRejectingTCP()
							container.NetOut(otherContainerNetwork.String(), tcpPort, "", garden.ProtocolTCP, -1, -1)
							ByRejectingUDP()
							ByRejectingICMP()
							ByAllowingTCP()
						})

						It("allows udp connections to that port", func() {
							container.NetOut(otherContainerNetwork.String(), 12345, "", garden.ProtocolUDP, -1, -1) // wrong port
							ByRejectingUDP()
							container.NetOut(otherContainerNetwork.String(), udpPort, "", garden.ProtocolUDP, -1, -1)
							ByRejectingICMP()
							ByRejectingTCP()
							ByAllowingUDP()
						})
					})

					Context("when a port range is specified", func() {
						It("allows tcp connections a port in that range", func() {
							container.NetOut(otherContainerNetwork.String(), 0, tcpPortRange, garden.ProtocolTCP, -1, -1)
							ByRejectingUDP()
							ByRejectingICMP()
							ByAllowingTCP()
						})

						It("allows udp connections a port in that range", func() {
							container.NetOut(otherContainerNetwork.String(), 0, tcpPortRange, garden.ProtocolUDP, -1, -1)
							ByRejectingTCP()
							ByRejectingICMP()
							ByAllowingUDP()
						})
					})
				})

				Describe("when no port or port is specified", func() {
					It("allows all TCP and blocks other protocols", func() {
						container.NetOut(otherContainerNetwork.String(), 0, "", garden.ProtocolTCP, -1, -1)
						ByAllowingTCP()
						ByRejectingICMP()
						ByRejectingUDP()
					})

					It("allows UDP and blocks other protocols", func() {
						container.NetOut(otherContainerNetwork.String(), 0, "", garden.ProtocolUDP, -1, -1)
						ByRejectingTCP()
						ByRejectingICMP()
						ByAllowingUDP()
					})
				})
			})

			Context("when the target address is inside ALLOW_NETWORKS", func() {
				BeforeEach(func() {
					containerNetwork = fmt.Sprintf("10.1%d.0.0/24", GinkgoParallelNode())
					denyRange = "10.0.0.0/8"
					allowRange = otherContainerNetwork.String()
				})

				It("allows connections", func() {
					ByAllowingTCP()
					ByAllowingUDP()
					ByAllowingICMP()
				})
			})

			Context("when the target address is in neither ALLOW_NETWORKS nor DENY_NETWORKS", func() {
				BeforeEach(func() {
					denyRange = "4.4.4.4/30"
					allowRange = "4.4.4.4/30"
					containerNetwork = fmt.Sprintf("10.1%d.0.0/24", GinkgoParallelNode())
				})

				It("allows connections", func() {
					ByAllowingTCP()
					ByAllowingUDP()
					ByAllowingICMP()
				})
			})
		})
	})
})

func tgzReader(path string) io.Reader {
	body, err := ioutil.ReadFile(path)
	Ω(err).ShouldNot(HaveOccurred())

	tmpdir, err := ioutil.TempDir("", "netdog")
	Ω(err).ShouldNot(HaveOccurred())

	tgzPath := filepath.Join(tmpdir, "netdog.tgz")

	archiver.CreateTarGZArchive(
		tgzPath,
		[]archiver.ArchiveFile{
			{
				Name: "./netdog",
				Body: string(body),
			},
		},
	)

	tgz, err := os.Open(tgzPath)
	Ω(err).ShouldNot(HaveOccurred())

	tarStream, err := gzip.NewReader(tgz)
	Ω(err).ShouldNot(HaveOccurred())

	return tarStream
}

func dumpIP() {
	cmd := exec.Command("ip", "a")
	op, err := cmd.CombinedOutput()
	Ω(err).ShouldNot(HaveOccurred())
	fmt.Println("IP status:\n", string(op))

	cmd = exec.Command("iptables", "--list")
	op, err = cmd.CombinedOutput()
	Ω(err).ShouldNot(HaveOccurred())
	fmt.Println("IP tables chains:\n", string(op))

	cmd = exec.Command("iptables", "--list-rules")
	op, err = cmd.CombinedOutput()
	Ω(err).ShouldNot(HaveOccurred())
	fmt.Println("IP tables rules:\n", string(op))
}