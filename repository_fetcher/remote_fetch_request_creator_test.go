package repository_fetcher_test

import (
	"errors"
	"net/url"

	. "github.com/cloudfoundry-incubator/garden-linux/repository_fetcher"
	"github.com/cloudfoundry-incubator/garden-linux/repository_fetcher/fake_pinger"
	"github.com/cloudfoundry-incubator/garden-linux/repository_fetcher/fakes"
	"github.com/docker/docker/registry"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RemoteFetchRequestCreator", func() {
	var (
		logger *lagertest.TestLogger

		creator              *RemoteFetchRequestCreator
		fakeRegistryProvider *fakes.FakeRegistryProvider
		fakeEndpointPinger   *fake_pinger.FakePinger
	)

	BeforeEach(func() {
		fakeRegistryProvider = new(fakes.FakeRegistryProvider)
		fakeEndpointPinger = new(fake_pinger.FakePinger)
		logger = lagertest.NewTestLogger("test")

		creator = &RemoteFetchRequestCreator{
			RegistryProvider: fakeRegistryProvider,
			Pinger:           fakeEndpointPinger,
		}
	})

	Context("when url path is empty", func() {
		It("returns an error", func() {
			_, err := creator.CreateFetchRequest(logger, &url.URL{Path: ""}, "", 0)
			Expect(err).To(Equal(ErrInvalidDockerURL))
		})
	})

	Context("when retrieving a session from the registry provider errors", func() {
		BeforeEach(func() {
			fakeRegistryProvider.ProvideRegistryReturns(nil, nil, errors.New("This is an error"))
		})

		It("returns the error, suitably wrapped", func() {
			parsedURL, err := url.Parse("some-scheme://some-registry:4444/some-repo")
			Expect(err).ToNot(HaveOccurred())

			_, err = creator.CreateFetchRequest(logger, parsedURL, "some-tag", 0)
			Expect(err).To(MatchError(ContainSubstring("repository_fetcher: RemoteFetchRequestCreator: could not fetch image some-repo from registry some-registry:4444: This is an error")))
		})
	})

	Context("when the provided registry is not available", func() {
		var parsedURL *url.URL

		BeforeEach(func() {
			var err error
			parsedURL, err = url.Parse("some-scheme://some-registry:4444/some-repo")
			Expect(err).ToNot(HaveOccurred())

			fakeRegistryProvider.ProvideRegistryReturns(nil, &registry.Endpoint{
				URL: parsedURL,
			}, nil)
			fakeEndpointPinger.PingReturns(registry.RegistryInfo{}, errors.New("This is an error"))
		})

		It("should return an error", func() {
			_, err := creator.CreateFetchRequest(logger, parsedURL, "some-tag", 0)
			Expect(err).To(MatchError(ContainSubstring("repository_fetcher: RemoteFetchRequestCreator: could not fetch image some-repo from registry some-registry:4444: This is an error")))
		})
	})

	Context("when the endpoint is provided", func() {
		var (
			returnedSession  *registry.Session
			returnedEndpoint *registry.Endpoint
			isStandalone     bool
		)

		JustBeforeEach(func() {
			returnedSession = &registry.Session{}
			returnedEndpoint = &registry.Endpoint{}
			fakeRegistryProvider.ProvideRegistryReturns(returnedSession, returnedEndpoint, nil)

			fakeEndpointPinger.PingReturns(registry.RegistryInfo{
				Standalone: isStandalone,
			}, nil)
		})

		Context("when the endpoint is not standalone", func() {
			BeforeEach(func() {
				isStandalone = false
			})
			It("prepends library prefore the remote path if the path does not contain a /", func() {
				fetchRequest, err := creator.CreateFetchRequest(logger, &url.URL{Path: "/somePath"}, "someTag", int64(987))
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeRegistryProvider.ProvideRegistryCallCount()).To(Equal(1))

				Expect(fetchRequest.Path).To(Equal("somePath"))
				Expect(fetchRequest.RemotePath).To(Equal("library/somePath"))
				Expect(fetchRequest.Tag).To(Equal("someTag"))
				Expect(fetchRequest.Session).To(Equal(returnedSession))
				Expect(fetchRequest.Endpoint).To(Equal(returnedEndpoint))
				Expect(fetchRequest.MaxSize).To(Equal(int64(987)))
			})

			It("does not prepends library prefore the remote path if the path does contain a /", func() {
				fetchRequest, err := creator.CreateFetchRequest(logger, &url.URL{Path: "/foo/somePath"}, "someTag", int64(987))
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeRegistryProvider.ProvideRegistryCallCount()).To(Equal(1))

				Expect(fetchRequest.Path).To(Equal("foo/somePath"))
				Expect(fetchRequest.RemotePath).To(Equal("foo/somePath"))
				Expect(fetchRequest.Tag).To(Equal("someTag"))
				Expect(fetchRequest.Session).To(Equal(returnedSession))
				Expect(fetchRequest.Endpoint).To(Equal(returnedEndpoint))
				Expect(fetchRequest.MaxSize).To(Equal(int64(987)))
			})
		})

		Context("when the endpoint IS standalone", func() {
			BeforeEach(func() {
				isStandalone = true
			})

			It("does not prepend library/ prefore the remote path", func() {
				fetchRequest, err := creator.CreateFetchRequest(logger, &url.URL{Path: "/foo/somePath"}, "someTag", int64(987))
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeRegistryProvider.ProvideRegistryCallCount()).To(Equal(1))

				Expect(fetchRequest.Path).To(Equal("foo/somePath"))
				Expect(fetchRequest.RemotePath).To(Equal("foo/somePath"))
				Expect(fetchRequest.Tag).To(Equal("someTag"))
				Expect(fetchRequest.Session).To(Equal(returnedSession))
				Expect(fetchRequest.Endpoint).To(Equal(returnedEndpoint))
				Expect(fetchRequest.MaxSize).To(Equal(int64(987)))
			})
		})
	})
})