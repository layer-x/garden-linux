package client_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/garden"
	. "github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/client/connection/fakes"
)

var _ = Describe("Client", func() {
	var client Client

	var fakeConnection *fakes.FakeConnection

	BeforeEach(func() {
		fakeConnection = new(fakes.FakeConnection)
	})

	JustBeforeEach(func() {
		client = New(fakeConnection)
	})

	Describe("Capacity", func() {
		BeforeEach(func() {
			fakeConnection.CapacityReturns(
				garden.Capacity{
					MemoryInBytes: 1111,
					DiskInBytes:   2222,
					MaxContainers: 42,
				},
				nil,
			)
		})

		It("sends a capacity request and returns the capacity", func() {
			capacity, err := client.Capacity()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(capacity.MemoryInBytes).Should(Equal(uint64(1111)))
			Ω(capacity.DiskInBytes).Should(Equal(uint64(2222)))
		})

		Context("when getting capacity fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.CapacityReturns(garden.Capacity{}, disaster)
			})

			It("returns the error", func() {
				_, err := client.Capacity()
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("BulkInfo", func() {
		expectedBulkInfo := map[string]garden.ContainerInfoEntry{
			"handle1": garden.ContainerInfoEntry{
				Info: garden.ContainerInfo{
					State: "container1State",
				},
			},
			"handle2": garden.ContainerInfoEntry{
				Info: garden.ContainerInfo{
					State: "container1State",
				},
			},
		}
		handles := []string{"handle1", "handle2"}

		It("gets info for the requested containers", func() {
			fakeConnection.BulkInfoReturns(expectedBulkInfo, nil)

			bulkInfo, err := client.BulkInfo(handles)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.BulkInfoCallCount()).Should(Equal(1))
			Ω(fakeConnection.BulkInfoArgsForCall(0)).Should(Equal(handles))
			Ω(bulkInfo).Should(Equal(expectedBulkInfo))
		})

		Context("when there is a error with the connection", func() {
			expectedBulkInfo := map[string]garden.ContainerInfoEntry{}

			BeforeEach(func() {
				fakeConnection.BulkInfoReturns(expectedBulkInfo, errors.New("Oh noes!"))
			})

			It("returns the error", func() {
				_, err := client.BulkInfo(handles)
				Ω(err).Should(MatchError("Oh noes!"))
			})
		})
	})

	Describe("Create", func() {
		It("sends a create request and returns a container", func() {
			spec := garden.ContainerSpec{
				RootFSPath: "/some/roofs",
			}

			fakeConnection.CreateReturns("some-handle", nil)

			container, err := client.Create(spec)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(container).ShouldNot(BeNil())

			Ω(fakeConnection.CreateArgsForCall(0)).Should(Equal(spec))

			Ω(container.Handle()).Should(Equal("some-handle"))
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.CreateReturns("", disaster)
			})

			It("returns it", func() {
				_, err := client.Create(garden.ContainerSpec{})
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Containers", func() {
		It("sends a list request and returns all containers", func() {
			fakeConnection.ListReturns([]string{"handle-a", "handle-b"}, nil)

			props := garden.Properties{"foo": "bar"}

			containers, err := client.Containers(props)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.ListArgsForCall(0)).Should(Equal(props))

			Ω(containers).Should(HaveLen(2))
			Ω(containers[0].Handle()).Should(Equal("handle-a"))
			Ω(containers[1].Handle()).Should(Equal("handle-b"))
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.ListReturns(nil, disaster)
			})

			It("returns it", func() {
				_, err := client.Containers(nil)
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Destroy", func() {
		It("sends a destroy request", func() {
			err := client.Destroy("some-handle")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.DestroyArgsForCall(0)).Should(Equal("some-handle"))
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.DestroyReturns(disaster)
			})

			It("returns it", func() {
				err := client.Destroy("some-handle")
				Ω(err).Should(Equal(disaster))
			})
		})

		Context("when the error is a 404", func() {
			notFound := connection.Error{404, ""}

			BeforeEach(func() {
				fakeConnection.DestroyReturns(notFound)
			})

			It("returns an ContainerNotFoundError with the requested handle", func() {
				err := client.Destroy("some-handle")
				Ω(err).Should(MatchError(garden.ContainerNotFoundError{"some-handle"}))
			})
		})
	})

	Describe("Lookup", func() {
		It("sends a list request", func() {
			fakeConnection.ListReturns([]string{"some-handle", "some-other-handle"}, nil)

			container, err := client.Lookup("some-handle")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(container.Handle()).Should(Equal("some-handle"))
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				fakeConnection.ListReturns([]string{"some-other-handle"}, nil)
			})

			It("returns ContainerNotFoundError", func() {
				_, err := client.Lookup("some-handle")
				Ω(err).Should(MatchError(garden.ContainerNotFoundError{"some-handle"}))
			})
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.ListReturns(nil, disaster)
			})

			It("returns it", func() {
				_, err := client.Lookup("some-handle")
				Ω(err).Should(Equal(disaster))
			})
		})
	})
})
