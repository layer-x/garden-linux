default: all

all:
	go build -o ${PWD}/out/wshd github.com/cloudfoundry-incubator/garden-linux/containerizer/wshd
	go build -o linux_backend/skeleton/lib/pivotter github.com/cloudfoundry-incubator/garden-linux/containerizer/system/pivotter
	go build -o ${PWD}/out/iodaemon github.com/cloudfoundry-incubator/garden-linux/iodaemon/cmd/iodaemon
	go build -o ${PWD}/out/wsh github.com/cloudfoundry-incubator/garden-linux/container_daemon/wsh
	CGO_ENABLED=0 go build -a -installsuffix static -o ${PWD}/out/initc github.com/cloudfoundry-incubator/garden-linux/containerizer/initc
	CGO_ENABLED=0 go build -a -installsuffix static -o ${PWD}/out/hook github.com/cloudfoundry-incubator/garden-linux/hook/hook
	go build -o ${PWD}/out/garden-linux -tags daemon github.com/cloudfoundry-incubator/garden-linux
	ln -s ${PWD}/out/wsh linux_backend/skeleton/bin/wsh
	ln -s ${PWD}/out/iodaemon linux_backend/skeleton/bin/iodaemon
	ln -s ${PWD}/out/wshd linux_backend/skeleton/bin/wshd
	ln -s ${PWD}/out/initc linux_backend/skeleton/bin/initc
	ln -s ${PWD}/out/hook linux_backend/skeleton/lib/hook
	cd linux_backend/src && make clean all
	cp linux_backend/src/oom/oom linux_backend/skeleton/bin
	cp linux_backend/src/nstar/nstar linux_backend/skeleton/bin
	
.PHONY: default
