BINDIR := bin
DRIVER_BASE_DIR := drivers


.PHONY: all
all: $(BINDIR)/disko

BASE_SOURCES = $(wildcard ./*.go)

DRIVER_NAMES = common cpm fat fat8 lbr minix unixv1 unixv6 unixv10
DRIVER_DIRECTORIES = $(addprefix drivers/,$(DRIVER_NAMES))
ALL_DRIVER_SOURCES = $(foreach d,$(DRIVER_DIRECTORIES),$(wildcard $(d)/*.go))

CLI_SOURCES = $(wildcard cmd/*.go)

ALL_SOURCES = $(ALL_DRIVER_SOURCES) $(CLI_SOURCES) $(BASE_SOURCES)


$(BINDIR)/disko: $(ALL_SOURCES) | $(BINDIR)
	go build -v -o $@ ./...


$(BINDIR):
	mkdir -p $@


.PHONY: clean
clean:
	$(RM) -r $(BINDIR)

.PHONY: test
test: $(ALL_SOURCES)
	go test -v ./...
