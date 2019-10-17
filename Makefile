LDFLAGS?=''
USER=$(shell id -u)
PKG=github.com/mgoltzsche/k8spkg

COMMIT_ID=$(shell git rev-parse HEAD)
COMMIT_TAG=$(shell git describe ${COMMIT_ID} || echo -n "dev")
COMMIT_DATE=$(shell git show -s --format=%ci ${COMMIT_ID})
LDFLAGS=-X "github.com/mgoltzsche/k8spkg/pkg/cmd.version=${COMMIT_TAG}" \
	-X "github.com/mgoltzsche/k8spkg/pkg/cmd.commit=${COMMIT_ID}" \
	-X "github.com/mgoltzsche/k8spkg/pkg/cmd.date=${COMMIT_DATE}"
BUILDTAGS?=

GOIMAGE=k8spkg-go
LITEIDEIMAGE=mgoltzsche/liteide:x36.1
DOCKERRUN=docker run --name k8spkg-build --rm \
		-v "$(shell pwd):/go/src/$(PKG)" \
		-w "/go/src/$(PKG)" \
		-u $(USER):$(USER) \
		-e HOME=/go \
		-e CGO_ENABLED=0
define GODOCKERFILE
FROM golang:1.12-alpine3.10
RUN apk add --update --no-cache make git
RUN go get golang.org/x/lint/golint
RUN go get -u github.com/go-bindata/go-bindata/...
endef
export GODOCKERFILE

all: clean build

build: golang-image
	$(DOCKERRUN) $(GOIMAGE) \
		make k8spkg BUILDTAGS=$(BUILDTAGS)

k8spkg: resources
	go build -a -ldflags '-s -w -extldflags "-static" $(LDFLAGS)' -tags '$(BUILDTAGS)' .

install:
	cp k8spkg /usr/local/bin/

test: resources
	go test -coverprofile coverage.out -cover ./...

coverage: test
	go tool cover -html=coverage.out -o coverage.html

resources:
	go-bindata crd/

clean:
	rm -f k8spkg coverage.out coverage.html

check-fmt-docker: golang-image
	$(DOCKERRUN) $(GOIMAGE) make check-fmt
check-fmt:
	cd "$$GOPATH/src" && MSGS="$$(gofmt -s -d $(shell go list ./pkg/...))" && [ ! "$$MSGS" ] || (echo "$$MSGS"; false)

lint-docker: golang-image
	$(DOCKERRUN) $(GOIMAGE) make lint
lint:
	# TODO: use in check target
	golint -set_exit_status $(shell go list ./...)

check: golang-image
	$(DOCKERRUN) $(GOIMAGE) \
		make clean k8spkg test check-fmt BUILDTAGS=$(BUILDTAGS)

coverage-report: golang-image
	$(DOCKERRUN) $(GOIMAGE) make coverage
	firefox coverage.html

vendor-update: golang-image
	mkdir -p .build-cache
	$(DOCKERRUN) -e GO111MODULE=on \
		--mount "type=bind,src=$(shell pwd)/.build-cache,dst=/go" \
		$(GOIMAGE) go mod vendor

golang-image:
	echo "$$GODOCKERFILE" | docker build --force-rm -t $(GOIMAGE) -

ide:
	mkdir -p .build-cache
	docker run -d --name liteide-k8spkg --rm \
		-e DISPLAY="$(shell echo $$DISPLAY)" \
		-e CHUSR=$(shell id -u):$(shell id -g) \
		-e GO111MODULE=on \
		-e GOFLAGS=' ' \
		--mount type=bind,src=/tmp/.X11-unix,dst=/tmp/.X11-unix \
		--mount type=bind,src=/etc/machine-id,dst=/etc/machine-id \
		--mount "type=bind,src=$(shell pwd)/.build-cache,dst=/go" \
		--mount "type=bind,src=$(shell pwd),dst=/go/src/$(PKG)" \
		"$(LITEIDEIMAGE)" \
		"/go/src/$(PKG)"
