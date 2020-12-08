package main

import (
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func getAbsPath(path string) (absPath string, err error) {
	if !filepath.IsAbs(path) {
		if path == "" {
			path = "."
		}

		return filepath.Abs(path)
	}

	return filepath.Clean(path), err
}

func checkIfProcessExist(pid int) bool {
	err := syscall.Kill(pid, syscall.Signal(0))
	return err == nil
}

func main() {
	var watchPath string
	var monitorPid int
	var tick int
	var verbose bool

	flag.StringVar(&watchPath, "path", ".", "filesystem path to watch for file changes")
	flag.BoolVar(&verbose, "verbose", false, "log verbose output")
	flag.IntVar(&monitorPid, "pid", -1, "pid to monitor and send signal")
	flag.IntVar(&tick, "tick", 5, "minimum duration between reloads in seconds")
	flag.Parse()

	watchPath, err := getAbsPath(watchPath)
	if err != nil {
		log.Fatal(err)
	}

	if verbose {
		log.Println("[Info] --verbose", verbose)
		log.Println("[Info] --path", watchPath)
		log.Println("[Info] --pid", monitorPid)
		log.Println("[Info] --tick", tick)
	}

	folderInfo, err := os.Stat(watchPath)
	if os.IsNotExist(err) {
		log.Fatal("folder does not exist: ", watchPath)
	}

	if !folderInfo.IsDir() {
		log.Fatal("file: ", watchPath, " is not a directory")
	}

	if !checkIfProcessExist(monitorPid) || monitorPid < 0 {
		log.Fatal("process id ", monitorPid, " does not exist")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	// Listen for shutdown signals
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Reload workers channel
	reload := make(chan bool, 1)
	ticker := time.NewTicker(time.Duration(tick) * time.Second)

	// Start event watcher worker coroutine
	go runWatcherWorker(watcher, verbose, reload)

	// Watch directories recursively
	err = addWatchedDirectoriesRecursively(watchPath, verbose, watcher)
	if err != nil {
		log.Fatal(err)
	}

	// Run reload worker
	go runReloadWorker(verbose, monitorPid, reload, ticker)

	// Watch for signals to gracefully shutdown
	go gracefulShutdown(watcher, verbose, quit, done, ticker, reload)

	if verbose {
		log.Println("[Lifecycle] Started swoole-bundle-fsnotify-reloader")
	}

	<-done
	if verbose {
		log.Println("[Lifecycle] swoole-bundle-fsnotify-reloader has stopped")
	}
}

func addWatchedDirectoriesRecursively(watchPath string, verbose bool, watcher *fsnotify.Watcher) (err error) {
	var dirs []string
	err = filepath.Walk(watchPath, visitRelevantDirectoriesOnly(&dirs, verbose))
	if err != nil {
		return err
	}

	for _, dirPath := range dirs {
		err = watcher.Add(dirPath)
		if err != nil {
			return err
		}

		if verbose {
			log.Println("[Info] Watching dir:", dirPath)
		}
	}

	return nil
}

func runReloadWorker(verbose bool, monitorPid int, reload <-chan bool, ticker *time.Ticker) {
	reloadRequests := 0
	for {
		select {
		case <-reload:
			reloadRequests++
		case <-ticker.C:
			if reloadRequests < 1 {
				continue
			}

			if verbose {
				log.Println("[Info] reload requested (times:", reloadRequests, ")")
			}

			err := syscall.Kill(monitorPid, syscall.SIGUSR1)
			if err != nil {
				log.Println("[Error]:", err)
			}
			reloadRequests = 0
		}
	}
}

func runWatcherWorker(watcher *fsnotify.Watcher, verbose bool, reload chan<- bool) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if verbose {
				log.Println("[Event]", event)
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println("[Info] modified file:", event.Name)
				if strings.HasSuffix(event.Name, ".php") ||
					strings.HasSuffix(event.Name, ".twig") ||
					strings.HasSuffix(event.Name, ".yaml") ||
					strings.HasSuffix(event.Name, ".yml") {
					reload <- true
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("[Error]", err)
		}
	}
}

func visitRelevantDirectoriesOnly(dirs *[]string, verbose bool) filepath.WalkFunc {
	return func(path string, info os.FileInfo, fileErr error) error {
		if !info.IsDir() {
			return nil
		}

		// skips var and vendor directories
		// skips "hidden" directories
		if info.Name() == "var" || info.Name() == "vendor" || strings.HasPrefix(info.Name(), ".") {
			if verbose {
				fmt.Println("[Info] Skipped dir:", path)
			}
			return filepath.SkipDir
		}

		*dirs = append(*dirs, path)
		return nil
	}
}

func gracefulShutdown(watcher *fsnotify.Watcher, verbose bool, quit <-chan os.Signal, done chan<- bool, ticker *time.Ticker, reload chan<- bool) {
	<-quit
	if verbose {
		log.Println("[Lifecycle] swoole-bundle-fsnotify-reloader is gracefully shutting down...")
	}

	err := watcher.Close()
	if err != nil {
		log.Println("[Error]", err)
	}

	close(reload)
	ticker.Stop()
	close(done)
}
