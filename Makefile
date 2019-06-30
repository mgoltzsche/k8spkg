LDFLAGS?=''
USER=$(shell id -u)
PKG=github.com/mgoltzsche/k8spkg

COMMIT_ID=$(shell git rev-parse HEAD)
COMMIT_TAG=$(shell git describe --exact-match ${COMMIT_ID} || echo -n "dev")
COMMIT_DATE=$(shell git show -s --format=%ci ${COMMIT_ID})
LDFLAGS=-X main.commit=${COMMIT_ID} -X main.version=${COMMIT_TAG} -X "main.date=${COMMIT_DATE}"
BUILDTAGS?=

CGO_ENABLED=1
GOIMAGE=k8spkg-golang
DOCKERRUN=docker run --name k8spkg-build --rm \
		-v "$(shell pwd)/.build-cache:/go" \
		-v "$(shell pwd):/go/src/$(PKG)" \
		-w "/go/src/$(PKG)" \
		-u $(USER):$(USER) \
		-e HOME=/go \
		-e GO111MODULE=on
define GODOCKERFILE
FROM golang:1.12-alpine3.9
RUN apk add --update --no-cache git
RUN rm -rf /go/*
endef
export GODOCKERFILE

all: clean k8spkg

k8spkg: golang-image
	$(DOCKERRUN) \
		-e GOOS=linux \
		-e CGO_ENABLED=0 \
		$(GOIMAGE) go build -a -ldflags '-s -w -extldflags "-static" $(LDFLAGS)' -tags '$(BUILDTAGS)' .

test: golang-image
	$(DOCKERRUN) \
		-e GOOS=linux \
		-e CGO_ENABLED=0 \
		$(GOIMAGE) go test -v ./...

golang-image:
	echo "$$GODOCKERFILE" | docker build --force-rm -t $(GOIMAGE) -

clean:
	rm -f k8spkg

ide:
	run-liteide . "$(PKG)"
