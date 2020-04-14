BINDATA_FLAGS = -fs -o internal/data/bindata.go -pkg data -prefix client/ client/...
BOOTSTRAP_VERSION = 4.4.1
VUE_VERSION = 2.6.11
VUE_ROUTER_VERSION = 3.1.6

.PHONY: all clean

default: binary

client/css/bootstrap.min.css:
	curl -fo $@ "https://stackpath.bootstrapcdn.com/bootstrap/$(BOOTSTRAP_VERSION)/css/bootstrap.min.css"

dev: client/css/bootstrap.min.css
	curl -fo client/js/vue.js "https://cdn.jsdelivr.net/npm/vue@$(VUE_VERSION)/dist/vue.js"
	curl -fo client/js/vue-router.js "https://cdn.jsdelivr.net/npm/vue-router@$(VUE_ROUTER_VERSION)/dist/vue-router.js"

	go mod download
	go-bindata -debug $(BINDATA_FLAGS)
	CGO_ENABLED=1 CompileDaemon -exclude-dir=.git -build="go build -o bin/cryptochat ./cmd/cryptochat" -command=bin/cryptochat \
    	-graceful-kill

binary: client/css/bootstrap.min.css
	curl -fo client/js/vue.js "https://cdn.jsdelivr.net/npm/vue@$(VUE_VERSION)/dist/vue.min.js"
	curl -fo client/js/vue-router.js "https://cdn.jsdelivr.net/npm/vue-router@$(VUE_ROUTER_VERSION)/dist/vue-router.min.js"

	go mod download
	go-bindata $(BINDATA_FLAGS)
	CGO_ENABLED=1 go build -ldflags '-s -w' -o bin/cryptochat ./cmd/cryptochat

clean:
	-rm -f bin/*
	-fm -f internal/data/*
