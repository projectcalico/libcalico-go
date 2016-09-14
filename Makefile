.PHONY: all test ut update-vendor

BUILD_CONTAINER_NAME=calico/calicoctl_build_container
BUILD_CONTAINER_MARKER=calicoctl_build_container.created

GO_FILES:=$(shell find calicoctl lib vendor -name '*.go')

CALICOCTL_VERSION?=$(shell git describe --tags --dirty --always)
CALICOCTL_BUILD_DATE?=$(shell date -u +'%FT%T%z')
CALICOCTL_GIT_REVISION?=$(shell git rev-parse --short HEAD)

LDFLAGS=-ldflags "-X github.com/tigera/libcalico-go/calicoctl/commands.VERSION=$(CALICOCTL_VERSION) \
	-X github.com/tigera/libcalico-go/calicoctl/commands.BUILD_DATE=$(CALICOCTL_BUILD_DATE) \
	-X github.com/tigera/libcalico-go/calicoctl/commands.GIT_REVISION=$(CALICOCTL_GIT_REVISION) -s -w"

default: all
all: test
test: ut

# Use this to populate the vendor directory after checking out the repository.
# To update upstream dependencies, delete the glide.lock file first.
vendor: glide.lock
	glide install -strip-vendor -strip-vcs --cache
	touch vendor

ut: update-tools bin/calicoctl
	./run-uts

.PHONY: force
force:
	true

bin/calicoctl: vendor $(GO_FILES) glide.*
	mkdir -p bin
	go build -o "$@" $(LDFLAGS) "./calicoctl/calicoctl.go"

release/calicoctl: clean force $(BUILD_CONTAINER_MARKER)
	mkdir -p bin release
	docker run --rm --privileged --net=host \
	-v ${PWD}:/go/src/github.com/tigera/libcalico-go:rw \
	-v ${PWD}/bin:/go/src/github.com/tigera/libcalico-go/bin:rw \
	$(BUILD_CONTAINER_NAME) make bin/calicoctl
	mv bin/calicoctl release/calicoctl
	rm -rf bin
	mv release/calicoctl release/calicoctl-$(CALICOCTL_VERSION)
	cd release && ln -sf calicoctl-$(CALICOCTL_VERSION) calicoctl

# Build calicoctl in a container.
build-containerized: $(BUILD_CONTAINER_MARKER)
	mkdir -p dist
	docker run -ti --rm --privileged --net=host \
	-e PLUGIN=calico \
	-v ${PWD}:/go/src/github.com/tigera/libcalico-go:rw \
	-v ${PWD}/dist:/go/src/github.com/tigera/libcalico-go/dist:rw \
	$(BUILD_CONTAINER_NAME) make bin/calicoctl

# Run the tests in a container. Useful for CI, Mac dev.
.PHONY: test-containerized
test-containerized: $(BUILD_CONTAINER_MARKER)
	docker run -ti --rm --privileged --net=host \
	-e PLUGIN=calico \
	-v ${PWD}:/go/src/github.com/tigera/libcalico-go:rw \
	$(BUILD_CONTAINER_NAME) make ut
	
$(BUILD_CONTAINER_MARKER): Dockerfile.build
	docker build -f Dockerfile.build -t $(BUILD_CONTAINER_NAME) .
	touch $@

# Install or update the tools used by the build
.PHONY: update-tools
update-tools:
	go get -u github.com/Masterminds/glide
	go get -u github.com/kisielk/errcheck
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/lint/golint
	go get -u github.com/onsi/ginkgo/ginkgo

# Etcd is used by the tests
run-etcd:
	@-docker rm -f calico-etcd
	docker run --detach \
	-p 2379:2379 \
	--name calico-etcd quay.io/coreos/etcd:v2.3.6 \
	--advertise-client-urls "http://127.0.0.1:2379,http://127.0.0.1:4001" \
	--listen-client-urls "http://0.0.0.0:2379,http://0.0.0.0:4001"

clean:
	rm -rf bin release $(BUILD_CONTAINER_MARKER)
