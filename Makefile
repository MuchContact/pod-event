all: build

PREFIX?=registry.aliyuncs.com/acs
FLAGS=
ARCH?=amd64
ALL_ARCHITECTURES=amd64 arm arm64 ppc64le s390x
ML_PLATFORMS=linux/amd64,linux/arm,linux/arm64,linux/ppc64le,linux/s390x


VERSION?=v1.2.0
GIT_COMMIT:=$(shell git rev-parse --short HEAD)


KUBE_EVENTER_LDFLAGS=-w -X events.com/pod-event/version.Version=$(VERSION) -X events.com/pod-event/version.GitCommit=$(GIT_COMMIT)

fmt:
	find . -type f -name "*.go" | grep -v "./vendor*" | xargs gofmt -s -w

build: clean
	GOARCH=$(ARCH) CGO_ENABLED=0 go build -ldflags "$(KUBE_EVENTER_LDFLAGS)" -o bin/pod-event events.com/pod

sanitize:
	hack/check_gofmt.sh
	hack/run_vet.sh

test-unit: clean sanitize build

ifeq ($(ARCH),amd64)
	GOARCH=$(ARCH) go test --test.short -race ./... $(FLAGS)
else
	GOARCH=$(ARCH) go test --test.short ./... $(FLAGS)
endif

test-unit-cov: clean sanitize build
	hack/coverage.sh

docker-container:
	docker build --pull -t $(PREFIX)/pod-event-$(ARCH):$(VERSION)-$(GIT_COMMIT)-aliyun -f Dockerfile .

clean:
	rm -f pod-event

.PHONY: all build sanitize test-unit test-unit-cov docker-container clean fmt
