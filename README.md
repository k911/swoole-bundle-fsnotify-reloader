# Swoole Bundle Fsnotify Reloader

Implementation of [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) for [k911/swoole-bundle](https://github.com/k911/swoole-bundle) HMR

## Building

```sh
CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -a -tags netgo -ldflags "-s -w" -o bin/fsnotify-reloader_linux_amd64 ./cmd/fsnotify-reloader/main.go
CGO_ENABLED=0 GOARCH=amd64 GOOS=darwin go build -a -tags netgo -ldflags "-s -w" -o bin/fsnotify-reloader_darwin_amd64 ./cmd/fsnotify-reloader/main.go
```