ifeq ($(OS),Windows_NT)
	HOSTOS=windows
	ifeq ($(PROCESSOR_ARCHITEW6432),AMD64)
		HOSTARCH=amd64
	else
		ifeq ($(PROCESSOR_ARCHITECTURE),AMD64)
			HOSTARCH=amd64
		else ifeq ($(PROCESSOR_ARCHITECTURE),x86)
			HOSTARCH=amd32
		endif
	endif
else
	UNAME_S := $(shell uname -s)
	ifeq ($(UNAME_S),Linux)
		HOSTOS=linux
	else ifeq ($(UNAME_S),Darwin)
		HOSTOS=darwin
	else
		@echo "Unknown OS: $(UNAME_S)"
		exit
	endif

	UNAME_M := $(shell uname -m)
	ifeq ($(UNAME_M),x86_64)
		HOSTARCH=amd64
	else ifneq ($(filter %86,$(UNAME_M)),)
		HOSTARCH=amd32
	else ifeq ($(UNAME_M),arm64)
		HOSTARCH=arm64
	else ifneq ($(filter arm%,$(UNAME_M)),)
		HOSTARCH=arm
	endif
endif

TARGETOS=$(HOSTOS)
ifdef MAKECMDGOALS
	ifeq ($(MAKECMDGOALS), $(filter $(MAKECMDGOALS), windows linux darwin))
		TARGETOS=$(MAKECMDGOALS)
	endif
endif

#GITTAG=$(shell git describe --tags $(shell git rev-list --tags --max-count=1))
GITCOMMIT=$(shell git log -1 --pretty=format:"%h")
BUILD_FLAGS=-a -ldflags "-w -s -X 'github.com/beatoz/beatoz-go/cmd/version.GitCommit=$(GITCOMMIT)'"

LOCAL_GOPATH = $(shell go env GOPATH)
BUILDDIR="./build/$(HOSTOS)"

.PHONY: all pbm $(TARGETOS) sfeeder deploy

all: pbm $(TARGETOS) sfeeder

$(TARGETOS):
	@echo Build beatoz for $(@) on $(UNAME_S)-$(UNAME_M)
ifeq ($(HOSTOS), windows)
	@set GOOS=$@& set GOARCH=$(HOSTARCH)& go build -o $(BUILDDIR)/beatoz.exe $(BUILD_FLAGS)  ./cmd/
else
	@GOOS=$@ GOARCH=$(HOSTARCH) go build -o $(BUILDDIR)/beatoz $(BUILD_FLAGS) ./cmd/
endif

sfeeder:
	@echo "Build SecretFeeder ..."
	@go build -o $(BUILDDIR)/sfeeder -ldflags "-s -w" ./sfeeder/sfeeder.go

install:
	@echo "Install binaries to $(LOCAL_GOPATH)/bin"
	@cp $(BUILDDIR)/* $(LOCAL_GOPATH)/bin
pbm:
	@echo Compile protocol messages
	@protoc --go_out=$(LOCAL_GOPATH)/src -I./protos/ account.proto
	@protoc --go_out=$(LOCAL_GOPATH)/src -I./protos/ gov_params.proto
	@protoc --go_out=$(LOCAL_GOPATH)/src -I./protos/ trx.proto
	@protoc --go_out=$(LOCAL_GOPATH)/src -I./protos/ reward.proto
	@protoc --go_out=$(LOCAL_GOPATH)/src --go-grpc_out=$(LOCAL_GOPATH)/src -I./sfeeder/protos secret_feeder.proto

deploy:
	@echo "Build deploy tar file"
	@mkdir -p .deploy
	@mkdir -p .tmp/deploy
	@cp ./scripts/deploy/cli/files/* .tmp/deploy/
	@cp $(BUILDDIR)/beatoz .tmp/deploy/

	@tar -czf .deploy/deploy.gz.tar -C .tmp/deploy .
	@tar -tzvf .deploy/deploy.gz.tar
	@rm -rf .tmp/deploy

# deploy:
# 	@echo Deploy...
# 	@sh -c scripts/deploy/deploy.sh

clean:
	@echo "Clean build..."
	@rm -rf $(BUILDDIR)

check:
	@echo "GOPATH": $(LOCAL_GOPATH)
	@echo "HOSTOS: $(HOSTOS)"
	@echo "HOSTARCH: $(HOSTARCH)"
	@echo "MAKECMDGOALS $(MAKECMDGOALS)"