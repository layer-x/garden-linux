package old

import (
	"flag"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/docker/docker/daemon/graphdriver"
	_ "github.com/docker/docker/daemon/graphdriver/aufs"
	_ "github.com/docker/docker/daemon/graphdriver/vfs"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/registry"
	"github.com/pivotal-golang/lager"

	"github.com/cloudfoundry-incubator/cf-debug-server"
	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/garden-linux/old/linux_backend"
	"github.com/cloudfoundry-incubator/garden-linux/old/linux_backend/container_pool"
	"github.com/cloudfoundry-incubator/garden-linux/old/linux_backend/container_pool/repository_fetcher"
	"github.com/cloudfoundry-incubator/garden-linux/old/linux_backend/container_pool/rootfs_provider"
	"github.com/cloudfoundry-incubator/garden-linux/old/linux_backend/network_pool"
	"github.com/cloudfoundry-incubator/garden-linux/old/linux_backend/port_pool"
	"github.com/cloudfoundry-incubator/garden-linux/old/linux_backend/quota_manager"
	"github.com/cloudfoundry-incubator/garden-linux/old/linux_backend/uid_pool"
	"github.com/cloudfoundry-incubator/garden-linux/old/sysconfig"
	"github.com/cloudfoundry-incubator/garden-linux/old/system_info"
	"github.com/cloudfoundry-incubator/garden-linux/old/volume"
	"github.com/cloudfoundry-incubator/garden/server"
	_ "github.com/cloudfoundry/dropsonde/autowire"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
)

var listenNetwork = flag.String(
	"listenNetwork",
	"unix",
	"how to listen on the address (unix, tcp, etc.)",
)

var listenAddr = flag.String(
	"listenAddr",
	"/tmp/garden.sock",
	"address to listen on",
)

var snapshotsPath = flag.String(
	"snapshots",
	"",
	"directory in which to store container state to persist through restarts",
)

var binPath = flag.String(
	"bin",
	"",
	"directory containing backend-specific scripts (i.e. ./create.sh)",
)

var depotPath = flag.String(
	"depot",
	"",
	"directory in which to store containers",
)

var overlaysPath = flag.String(
	"overlays",
	"",
	"directory in which to store containers mount points",
)

var rootFSPath = flag.String(
	"rootfs",
	"",
	"directory of the rootfs for the containers",
)

var globalVolumesPath = flag.String(
	"globalVolumesPath",
	"",
	"directory in which to store volumes",
)

var disableQuotas = flag.Bool(
	"disableQuotas",
	false,
	"disable disk quotas",
)

var containerGraceTime = flag.Duration(
	"containerGraceTime",
	0,
	"time after which to destroy idle containers",
)

var networkPool = flag.String(
	"networkPool",
	"10.254.0.0/22",
	"network pool CIDR for containers; each container will get a /30",
)

var portPoolStart = flag.Uint(
	"portPoolStart",
	61001,
	"start of ephemeral port range used for mapped container ports",
)

var portPoolSize = flag.Uint(
	"portPoolSize",
	5000,
	"size of port pool used for mapped container ports",
)

var uidPoolStart = flag.Uint(
	"uidPoolStart",
	10000,
	"start of per-container user ids",
)

var uidPoolSize = flag.Uint(
	"uidPoolSize",
	256,
	"size of the uid pool",
)

var denyNetworks = flag.String(
	"denyNetworks",
	"",
	"CIDR blocks representing IPs to blacklist",
)

var allowNetworks = flag.String(
	"allowNetworks",
	"",
	"CIDR blocks representing IPs to whitelist",
)

var graphRoot = flag.String(
	"graph",
	"/var/lib/garden-docker-graph",
	"docker image graph",
)

var dockerRegistry = flag.String(
	"registry",
	registry.IndexServerAddress(),
	"docker registry API endpoint",
)

var tag = flag.String(
	"tag",
	"",
	"server-wide identifier used for 'global' configuration",
)

func Main() {
	flag.Parse()

	cf_debug_server.Run()

	runtime.GOMAXPROCS(runtime.NumCPU())

	logger := cf_lager.New("garden-linux")

	if *binPath == "" {
		missing("-bin")
	}

	if *depotPath == "" {
		missing("-depot")
	}

	if *overlaysPath == "" {
		missing("-overlays")
	}

	uidPool := uid_pool.New(uint32(*uidPoolStart), uint32(*uidPoolSize))

	_, ipNet, err := net.ParseCIDR(*networkPool)
	if err != nil {
		logger.Fatal("malformed-network-pool", err)
	}

	networkPool := network_pool.New(ipNet)

	// TODO: use /proc/sys/net/ipv4/ip_local_port_range by default (end + 1)
	portPool := port_pool.New(uint32(*portPoolStart), uint32(*portPoolSize))

	config := sysconfig.NewConfig(*tag)

	runner := sysconfig.NewRunner(config, linux_command_runner.New())

	quotaManager := quota_manager.New(runner, getMountPoint(logger, *depotPath), *binPath)

	if *disableQuotas {
		quotaManager.Disable()
	}

	if err := os.MkdirAll(*graphRoot, 0755); err != nil {
		logger.Fatal("failed-to-create-graph-directory", err)
	}

	graphDriver, err := graphdriver.New(*graphRoot, nil)
	if err != nil {
		logger.Fatal("failed-to-construct-graph-driver", err)
	}

	graph, err := graph.NewGraph(*graphRoot, graphDriver)
	if err != nil {
		logger.Fatal("failed-to-construct-graph", err)
	}

	reg, err := registry.NewSession(nil, nil, *dockerRegistry, true)
	if err != nil {
		logger.Fatal("failed-to-construct-registry", err)
	}

	repoFetcher := repository_fetcher.Retryable{repository_fetcher.New(reg, graph)}

	rootFSProviders := map[string]rootfs_provider.RootFSProvider{
		"":       rootfs_provider.NewOverlay(*binPath, *overlaysPath, *rootFSPath, runner),
		"docker": rootfs_provider.NewDocker(repoFetcher, graphDriver),
	}

	containerPool := container_pool.New(
		logger,
		*binPath,
		*depotPath,
		*globalVolumesPath,
		config,
		rootFSProviders,
		uidPool,
		networkPool,
		portPool,
		strings.Split(*denyNetworks, ","),
		strings.Split(*allowNetworks, ","),
		runner,
		quotaManager,
	)

	volumePool := volume.NewPool(*globalVolumesPath)

	systemInfo := system_info.NewProvider(*depotPath)

	backend := linux_backend.New(logger, containerPool, volumePool, systemInfo, *snapshotsPath)

	err = backend.Setup()
	if err != nil {
		logger.Fatal("failed-to-set-up-backend", err)
	}

	graceTime := *containerGraceTime

	gardenServer := server.New(*listenNetwork, *listenAddr, graceTime, backend, logger)

	err = gardenServer.Start()
	if err != nil {
		logger.Fatal("failed-to-start-server", err)
	}

	logger.Info("started", lager.Data{
		"network": *listenNetwork,
		"addr":    *listenAddr,
	})

	signals := make(chan os.Signal, 1)

	go func() {
		<-signals
		gardenServer.Stop()
		os.Exit(0)
	}()

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	select {}
}

func getMountPoint(logger lager.Logger, depotPath string) string {
	//	dfOut := new(bytes.Buffer)
	//
	//	df := exec.Command("df", depotPath)
	//	df.Stdout = dfOut
	//	df.Stderr = os.Stderr
	//
	//	err := df.Run()
	//	if err != nil {
	//		logger.Fatal("failed-to-get-mount-info", err)
	//	}
	//
	//	dfOutputWords := strings.Split(string(dfOut.Bytes()), " ")
	//
	//	return strings.Trim(dfOutputWords[len(dfOutputWords)-1], "\n")

	searchingPath := depotPath
	for {
		absPath, err := filepath.Abs(searchingPath)
		if err != nil {
			logger.Fatal("failed-to-resolve-path", err)
		}

		isMP, err := isMountPoint(logger, absPath)
		if err != nil {
			logger.Fatal("failed-to-check-if-mount-point", err)
		}

		if isMP {
			return searchingPath
		}

		searchingPath = filepath.Join(searchingPath, "..")
	}

	return "/"
}

func isMountPoint(logger lager.Logger, path string) (bool, error) {
	target, err := os.Stat(path)
	if err != nil {
		logger.Error("failed-to-stat-path", err)
		return false, err
	}

	parent, err := os.Stat(filepath.Join(path, ".."))
	if err != nil {
		logger.Error("failed-to-stat-parent", err)
		return false, err
	}

	targetStat_t := target.Sys().(*syscall.Stat_t)
	parentStat_t := parent.Sys().(*syscall.Stat_t)

	return ((targetStat_t.Dev == parentStat_t.Dev && targetStat_t.Ino == parentStat_t.Ino) ||
		(targetStat_t.Dev != parentStat_t.Dev)), nil
}

func missing(flagName string) {
	println("missing " + flagName)
	println()
	flag.Usage()
}
