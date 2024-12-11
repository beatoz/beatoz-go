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
BUILDDIR=./build/$(HOSTOS)

OUTPUT=$(BUILDDIR)/beatoz
ifeq ($(HOSTOS), windows)
	OUTPUT=$(BUILDDIR)/beatoz.exe
endif

BUILD_YN="Y"
ifneq ($(wildcard $(OUTPUT)),)
	commit0=$(word 2, $(subst -, ,$(subst @, ,$(shell $(OUTPUT) version))))
	ifeq ($(commit0),$(GITCOMMIT))
		BUILD_YN="N"
	endif
endif

.PHONY: all pbm $(TARGETOS) deploy

all: pbm $(TARGETOS)

$(TARGETOS):
ifeq ($(BUILD_YN),"Y")
	@echo Build beatoz for $(@) on $(UNAME_S)-$(UNAME_M)
ifeq ($(HOSTOS),windows)
	@set GOOS=$@& set GOARCH=$(HOSTARCH)& go build -o $(OUTPUT) $(BUILD_FLAGS)  ./cmd/
else
	@GOOS=$@ GOARCH=$(HOSTARCH) go build -o $(OUTPUT) $(BUILD_FLAGS) ./cmd/
endif
else
	@echo "The last version ($(shell $(OUTPUT) version)) has already been built."
endif

pbm:
	@echo Compile protocol messages
	@protoc --go_out=$(LOCAL_GOPATH)/src -I./protos/ account.proto
	@protoc --go_out=$(LOCAL_GOPATH)/src -I./protos/ gov_params.proto
	@protoc --go_out=$(LOCAL_GOPATH)/src -I./protos/ trx.proto
	@protoc --go_out=$(LOCAL_GOPATH)/src -I./protos/ reward.proto

install:
	@echo "Install binaries to $(LOCAL_GOPATH)/bin"
	@cp $(BUILDDIR)/* $(LOCAL_GOPATH)/bin

clean:
	@echo "Clean build..."
	@rm -rf $(BUILDDIR)

check:
	@echo "GOPATH": $(LOCAL_GOPATH)
	@echo "HOSTOS: $(HOSTOS)"
	@echo "HOSTARCH: $(HOSTARCH)"
	@echo "MAKECMDGOALS $(MAKECMDGOALS)"