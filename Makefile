BINDIR:=bin

all: $(BINDIR)/disko $(BINDIR)/fat
.PHONY: all

$(BINDIR)/disko: api.go errors.go
	go build -o $@ $<

FAT_DIR := drivers/fat
FAT_SOURCES := $(wildcard $(FAT_DIR)/*.go)

$(BINDIR)/fat: $(FAT_SOURCES)
	go build -o $@ $(FAT_SOURCES)

clean:
	rm -rf $(BINDIR)
.PHONY: clean
