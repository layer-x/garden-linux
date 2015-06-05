package quota_manager_test

import (
	"errors"
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-linux/linux_container/quota_manager"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
)

var _ = Describe("btrfs quota manager", func() {
	var fakeRunner *fake_command_runner.FakeCommandRunner
	var logger *lagertest.TestLogger
	var quotaManager *quota_manager.BtrfsQuotaManager
	var subvolumePath string
	var qgroupShowResponse []byte
	var qgroupShowError error

	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		logger = lagertest.NewTestLogger("test")
		quotaManager = quota_manager.New(fakeRunner)
		subvolumePath = "/some/volume/path"
	})

	JustBeforeEach(func() {
		fakeRunner.WhenRunning(
			fake_command_runner.CommandSpec{
				Path: "sh",
				Args: []string{"-c", fmt.Sprintf("btrfs qgroup show -rF --raw %s", subvolumePath)},
			},
			func(cmd *exec.Cmd) error {
				cmd.Stdout.Write(qgroupShowResponse)
				return qgroupShowError
			},
		)
	})

	Describe("setting quotas", func() {
		limits := garden.DiskLimits{
			ByteSoft: 1,
			ByteHard: 2,

			InodeSoft: 11,
			InodeHard: 12,
		}

		Context("when the subvolume exists", func() {
			BeforeEach(func() {
				qgroupShowResponse = []byte(
					`qgroupid         rfer         excl     max_rfer
--------         ----         ----     --------
0/257           16384        16384  16384
`)
				qgroupShowError = nil
			})

			It("executes qgroup limit with the correct qgroup id", func() {
				err := quotaManager.SetLimits(logger, subvolumePath, limits)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeRunner).To(HaveExecutedSerially(
					fake_command_runner.CommandSpec{
						Path: "btrfs",
						Args: []string{"qgroup", "limit", "2", "0/257", subvolumePath},
					},
				))
			})

			Context("when blocks are given", func() {
				limits := garden.DiskLimits{
					BlockSoft: 10,
					BlockHard: 20,
				}

				It("executes qgroup limit with them converted to bytes", func() {
					err := quotaManager.SetLimits(logger, subvolumePath, limits)

					Expect(err).ToNot(HaveOccurred())

					Expect(fakeRunner).To(HaveExecutedSerially(
						fake_command_runner.CommandSpec{
							Path: "btrfs",
							Args: []string{
								"qgroup", "limit", "20480", "0/257", subvolumePath},
						},
					))
				})
			})

			Context("when executing qgroup limit fails", func() {
				nastyError := errors.New("oh no!")

				BeforeEach(func() {
					fakeRunner.WhenRunning(
						fake_command_runner.CommandSpec{
							Path: "btrfs",
						}, func(*exec.Cmd) error {
							return nastyError
						},
					)
				})

				It("returns the error", func() {
					err := quotaManager.SetLimits(logger, subvolumePath, limits)
					Expect(err).To(MatchError("quota_manager: failed to apply limit: oh no!"))
				})
			})

			Context("when btrfs returns malformed results", func() {
				BeforeEach(func() {
					qgroupShowResponse = []byte("What!! Oh no.. Wait.")
					qgroupShowError = nil
				})

				It("returns the error", func() {
					_, err := quotaManager.GetLimits(logger, subvolumePath)
					Expect(err).To(MatchError(ContainSubstring("quota_manager: parse quota info")))
				})
			})
		})

		Context("when the desired subvolume cannot be found", func() {
			BeforeEach(func() {
				qgroupShowResponse = []byte("")
				qgroupShowError = errors.New("exit status 3")
			})

			It("returns an error", func() {
				err := quotaManager.SetLimits(logger, subvolumePath, limits)
				Expect(err).To(MatchError(ContainSubstring("quota_manager: run quota info: exit status 3")))
			})
		})

		Context("when quotas are disabled", func() {
			BeforeEach(func() {
				quotaManager.Disable()
			})

			It("runs nothing", func() {
				err := quotaManager.SetLimits(logger, subvolumePath, limits)

				Expect(err).ToNot(HaveOccurred())

				for _, cmd := range fakeRunner.ExecutedCommands() {
					Expect(cmd.Path).ToNot(Equal("btrfs"))
				}
			})
		})
	})

	Describe("getting quotas limits", func() {
		BeforeEach(func() {
			qgroupShowResponse = []byte(
				`qgroupid         rfer         excl     max_rfer
--------         ----         ----     --------
0/257           16384        16384  1000000
`)
			qgroupShowError = nil
		})

		It("gets current quotas using btrfs", func() {
			limits, err := quotaManager.GetLimits(logger, subvolumePath)
			Expect(err).ToNot(HaveOccurred())

			Expect(limits.ByteSoft).To(Equal(uint64(1000000)))
			Expect(limits.ByteHard).To(Equal(uint64(1000000)))
		})

		Context("when quotas are disabled", func() {
			BeforeEach(func() {
				quotaManager.Disable()
			})

			It("runs nothing", func() {
				limits, err := quotaManager.GetLimits(logger, subvolumePath)
				Expect(err).ToNot(HaveOccurred())

				Expect(limits).To(BeZero())
				Expect(fakeRunner.ExecutedCommands()).To(HaveLen(0))
			})
		})
	})

	Describe("getting usage", func() {
		BeforeEach(func() {
			qgroupShowResponse = []byte(
				`qgroupid         rfer         excl     max_rfer
--------         ----         ----     --------
0/257           10485760        16384     16384
`)
			qgroupShowError = nil
		})

		Context("when btrfs succeeds", func() {
			BeforeEach(func() {
				fakeRunner.WhenRunning(
					fake_command_runner.CommandSpec{
						Path: "btrfs",
						Args: []string{"quota", "rescan", subvolumePath},
					},
					func(cmd *exec.Cmd) error {
						cmd.Stdout.Write([]byte("\n"))
						return nil
					},
				)
			})

			It("reports the disk usage", func() {
				usage, err := quotaManager.GetUsage(logger, subvolumePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(usage).To(Equal(garden.ContainerDiskStat{
					BytesUsed:  uint64(10 * 1024 * 1024),
					InodesUsed: uint64(0),
				}))
			})
		})

		Context("when quotas are disabled", func() {
			BeforeEach(func() {
				quotaManager.Disable()
			})

			It("runs nothing", func() {
				usage, err := quotaManager.GetUsage(logger, subvolumePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(usage).To(BeZero())

				for _, cmd := range fakeRunner.ExecutedCommands() {
					Expect(cmd.Path).ToNot(Equal("btrfs"))
				}
			})
		})
	})
})