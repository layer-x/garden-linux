default: all

all:
	go build -o ${PWD}/out/garden-linux -tags daemon github.com/cloudfoundry-incubator/garden-linux
	cd linux_backend/src && make clean all
	cd linux_backend/src && make clean
	
.PHONY: default
