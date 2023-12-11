CLANG = clang
CC = $(CLANG)

ARCH := $(shell uname -m)
ARCH := $(subst x86_64,amd64,$(ARCH))
GOARCH := $(ARCH)

GIT = $(shell which git || /bin/false)

CFLAGS = 
LDFLAGS =

CGO_CFLAGS = '-ggdb -gdwarf -O2 -Wall -fpie'
CGO_LDFLAGS =
CGO_EXTLDFLAGS = '-w -extldflags "-static"'

## program

.PHONY: $(PROGRAM)
.PHONY: $(PROGRAM).bpf.c

PROGRAM = ghtimecardator

all: $(PROGRAM)

## GO example

.PHONY: $(PROGRAM).go
.PHONY: $(PROGRAM)

$(PROGRAM): $(PROGRAM).go
	@CC=$(CLANG) \
	CGO_CFLAGS=$(CGO_CFLAGS) \
	CGO_LDFLAGS=$(CGO_LDFLAGS) \
	GOARCH=$(GOARCH) \
		go build \
		-tags netgo -ldflags $(CGO_EXTLDFLAGS) \
		-o $(PROGRAM) .

## clean

clean:
	@rm -f $(PROGRAM)
