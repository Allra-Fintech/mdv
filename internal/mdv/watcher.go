package mdv

import (
	"log"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// WatchFile starts an fsnotify watcher on both the file and its parent directory.
// This handles atomic-write editors (Vim, JetBrains) that replace the file inode.
// Any Write or Create event for the watched file triggers hub.Broadcast().
// The function blocks until the watcher is closed; run it in a goroutine.
func WatchFile(path string, hub *Hub) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		log.Printf("watcher: cannot resolve path %q: %v", path, err)
		return
	}
	dir := filepath.Dir(absPath)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("watcher: cannot create watcher: %v", err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add(absPath); err != nil {
		log.Printf("watcher: cannot watch file %q: %v", absPath, err)
	}
	if err := watcher.Add(dir); err != nil {
		log.Printf("watcher: cannot watch dir %q: %v", dir, err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				if filepath.Ext(event.Name) == ".md" || event.Name == absPath {
					hub.Broadcast()
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("watcher: error: %v", err)
		}
	}
}
