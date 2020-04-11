BINDATA_FLAGS = -fs -o internal/data/bindata.go -pkg data client/...

.PHONY: all clean

default: binary

dev:
	go mod download
	go-bindata -debug $(BINDATA_FLAGS)
	CompileDaemon -exclude-dir=.git -build="go build -o bin/cryptochat ./cmd/cryptochat" -command=bin/cryptochat \
    	-graceful-kill

binary:
	go mod download
	go-bindata $(BINDATA_FLAGS)
	go build -ldflags '-s -w' -o bin/cryptochat ./cmd/cryptochat

clean:
	-rm -f bin/*
	-fm -f internal/data/*
