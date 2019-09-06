MAKEOPTS = $(MAKEOPTS)
GIT_COMMIT := $(shell git rev-list -1 HEAD)
VERSION := 0.2.1
BUILDOPTS :=$(BUILDOPTS) -ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT)"

.PHONY: install all clean

install: all
	go install $(BUILDOPTS) .

all:
	go build $(BUILDOPTS) .

clean:
	go clean .

