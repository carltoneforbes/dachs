.PHONY: build install clean

build:
	go build -o dachs .
	@if [ "$$(uname)" = "Darwin" ]; then codesign --force --sign - dachs; fi

install: build
	@mkdir -p $${GOPATH:-$$HOME/go}/bin
	cp dachs $${GOPATH:-$$HOME/go}/bin/dachs
	@if [ "$$(uname)" = "Darwin" ]; then codesign --force --sign - $${GOPATH:-$$HOME/go}/bin/dachs; fi
	@echo "Installed to $${GOPATH:-$$HOME/go}/bin/dachs"

clean:
	rm -f dachs
