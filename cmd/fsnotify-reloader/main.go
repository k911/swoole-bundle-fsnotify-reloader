package main

import (
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func getAbsPath(path string) (absPath string, err error) {
	if !filepath.IsAbs(path) {
		if path == "" {
			path = "."
		}

		return filepath.Abs(path)
	}

	return path, err
}

func main() {
	var watchPath string
	var verbose bool

	flag.StringVar(&watchPath, "path", ".", "filesystem path to watch for file changes")
	flag.BoolVar(&verbose, "verbose", false, "log verbose output")
	flag.Parse()

	watchPath, err := getAbsPath(watchPath)
	if err != nil {
		log.Fatal(err)
	}

	if verbose {
		log.Println("[Info] --path", watchPath)
	}

	folderInfo, err := os.Stat(watchPath)
	if os.IsNotExist(err) {
		log.Fatal("folder does not exist: ", watchPath)
	}
	if !folderInfo.IsDir() {
		log.Fatal("file: ", watchPath, " is not a directory")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
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
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("[Error]", err)
			}
		}
	}()

	var dirs []string
	err = filepath.Walk(watchPath, visitRelevantDirectoriesOnly(&dirs, verbose))

	for _, dirPath := range dirs {
		err = watcher.Add(dirPath)
		if err != nil {
			log.Fatal(err)
		}

		if verbose {
			log.Println("[Info] Watching dir:", dirPath)
		}
	}

	<-done
}

func visitRelevantDirectoriesOnly(dirs *[]string, verbose bool) filepath.WalkFunc {
	return func(path string, info os.FileInfo, fileErr error) error {
		if !info.IsDir() {
			return nil
		}

		// skips var directory
		// skips "hidden" directories
		if info.Name() == "var" || strings.HasPrefix(info.Name(), ".") {
			if verbose {
				fmt.Println("[Info] Skipped dir:", path)
			}
			return filepath.SkipDir
		}

		*dirs = append(*dirs, path)
		return nil
	}
}
