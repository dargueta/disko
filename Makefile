BINDIR := bin

DISKO_BIN = $(BINDIR)/disko
ZIPIMAGE_BIN = $(BINDIR)/zipimage
UNZIPIMAGE_BIN = $(BINDIR)/unzipimage

COMPRESSION_SOURCES = $(wildcard utilities/compression/*.go)


.PHONY: all cli disko zipimage unzipimage

all: disko
cli: disko zipimage unzipimage
disko: $(DISKO_BIN)
zipimage: $(ZIPIMAGE_BIN)
unzipimage: $(UNZIPIMAGE_BIN)

.envrc:
	echo "export PATH=$(PATH):$(CURDIR)/$(BINDIR)" > $@
	if which direnv 2>&1 >/dev/null ; then direnv allow; fi


$(DISKO_BIN): $(ALL_SOURCES) | $(BINDIR)
	go build -v -o $@ ./...

$(ZIPIMAGE_BIN): $(COMPRESSION_SOURCES) cmd/zipimage/main.go | $(BINDIR)
	go build -v -o $@ ./cmd/zipimage

$(UNZIPIMAGE_BIN): $(COMPRESSION_SOURCES) cmd/unzipimage/main.go | $(BINDIR)
	go build -v -o $@ ./cmd/unzipimage


$(BINDIR):
	mkdir -p $@


.PHONY: clean
clean:
	$(RM) -r $(BINDIR)

.PHONY: test
test: $(ALL_SOURCES)
	go test -v -shuffle on -cover ./...
