BINDIR := bin

.PHONY: all
all: $(BINDIR)/disko $(BINDIR)/fat

$(BINDIR)/disko: api.go errors.go
	go build -o $@ $<

FAT_DIR := drivers/fat
FAT_SOURCES := $(wildcard $(FAT_DIR)/*.go)

$(BINDIR)/fat: $(FAT_SOURCES)
	go build -o $@ $(FAT_SOURCES)

.PHONY: clean
clean:
	$(RM) -r $(BINDIR)
