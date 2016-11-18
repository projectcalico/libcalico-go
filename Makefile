.PHONY: all test

GLIDE_CONTAINER_NAME?=dockerepo/glide
TEST_CONTAINER_NAME=calico/libcalico_test_container
TEST_CONTAINER_MARKER=libcalico_test_container.created
VENDOR_CREATED=vendor/.vendor.created

GO_FILES:=$(shell find lib -name '*.go')

default: all
all: test
test: ut

.PHONY: vendor
## Use this to populate the vendor directory after checking out the repository.
## To update upstream dependencies, delete the glide.lock file first.
vendor: glide.lock $(VENDOR_CREATED)

# If glide.lock is missing then do the vendoring which will create the lock file
# (delete the vendor created flag if it exists to force vendoring).
glide.lock:
	rm -f $(VENDOR_CREATED)
	$(MAKE) $(VENDOR_CREATED)

# Perform the vendoring.  This writes a created flag file only after the vendoring
# completed successfully - this allows us to easily re-run in the event of a partially
# incomplete vendoring.
$(VENDOR_CREATED):
	docker run --rm -v ${PWD}:/go/src/github.com/projectcalico/libcalico-go:rw \
      --entrypoint /bin/sh $(GLIDE_CONTAINER_NAME) -e -c ' \
        cd /go/src/github.com/projectcalico/libcalico-go && \
        glide install -strip-vendor && \
        chown -R $(shell id -u):$(shell id -u) vendor'
	touch $(VENDOR_CREATED)

.PHONY: ut
## Run the UTs locally.  This requires a local etcd to be running.
ut:
	# Run tests in random order find tests recursively (-r).
	ginkgo -cover -r --skipPackage vendor

	@echo
	@echo '+==============+'
	@echo '| All coverage |'
	@echo '+==============+'
	@echo
	@find . -iname '*.coverprofile' | xargs -I _ go tool cover -func=_

	@echo
	@echo '+==================+'
	@echo '| Missing coverage |'
	@echo '+==================+'
	@echo
	@find . -iname '*.coverprofile' | xargs -I _ go tool cover -func=_ | grep -v '100.0%'

.PHONY: test-containerized
## Run the tests in a container. Useful for CI, Mac dev.
test-containerized: vendor run-etcd $(TEST_CONTAINER_MARKER)
	docker run --rm --privileged --net=host \
	-e PLUGIN=calico \
	-v ${PWD}:/go/src/github.com/projectcalico/libcalico-go:rw \
	$(TEST_CONTAINER_NAME) bash -c 'make ut && chown $(shell id -u):$(shell id -g) -R ./vendor'

## Install or update the tools used by the build
.PHONY: update-tools
update-tools:
	go get -u github.com/Masterminds/glide
	go get -u github.com/kisielk/errcheck
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/lint/golint
	go get -u github.com/onsi/ginkgo/ginkgo

## Run etcd as a container
run-etcd:
	@-docker rm -f calico-etcd
	docker run --detach \
	-p 2379:2379 \
	--name calico-etcd quay.io/coreos/etcd:v2.3.6 \
	--advertise-client-urls "http://127.0.0.1:2379,http://127.0.0.1:4001" \
	--listen-client-urls "http://0.0.0.0:2379,http://0.0.0.0:4001"

$(TEST_CONTAINER_MARKER):
	docker build -f Dockerfile -t $(BUILD_CONTAINER_NAME) .
	touch $@

.PHONY: clean
clean:
	find . -name '*.coverprofile' -type f -delete
	rm -rf vendor
	-rm $(TEST_CONTAINER_MARKER)

.PHONY: help
## Display this help text
help: # Some kind of magic from https://gist.github.com/rcmachado/af3db315e31383502660
	$(info Available targets)
	@awk '/^[a-zA-Z\-\_0-9\/]+:/ {                                      \
		nb = sub( /^## /, "", helpMsg );                                \
		if(nb == 0) {                                                   \
			helpMsg = $$0;                                              \
			nb = sub( /^[^:]*:.* ## /, "", helpMsg );                   \
		}                                                               \
		if (nb)                                                         \
			printf "\033[1;31m%-" width "s\033[0m %s\n", $$1, helpMsg;  \
	}                                                                   \
	{ helpMsg = $$0 }'                                                  \
	width=20                                                            \
	$(MAKEFILE_LIST)
