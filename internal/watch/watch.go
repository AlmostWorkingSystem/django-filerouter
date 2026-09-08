package watch

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Event is a qualifying filesystem change the caller should act on.
type Event struct {
	Path string
	Op   fsnotify.Op
}

// filter replicates WatchDogReloader's should_skip_event debounce plus its
// on_any_event qualifying rule (modules/core/management/commands/server.py:33-88):
// only Create/Remove/Rename of a .py file under a path containing "/api/",
// never _routes.py, and never the same path twice within `debounce`.
type filter struct {
	debounce time.Duration
	mu       sync.Mutex
	last     map[string]time.Time
}

func newFilter(debounce time.Duration) *filter {
	return &filter{debounce: debounce, last: map[string]time.Time{}}
}

func (f *filter) shouldTrigger(ev fsnotify.Event) bool {
	if !strings.HasSuffix(ev.Name, ".py") {
		return false
	}
	if strings.HasSuffix(ev.Name, "_routes.py") {
		return false
	}
	if !strings.Contains(ev.Name, "/api/") {
		return false
	}
	if ev.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	if last, ok := f.last[ev.Name]; ok && now.Sub(last) < f.debounce {
		return false
	}
	f.last[ev.Name] = now
	return true
}

// Watch recursively watches root and calls onQualifying for every
// debounced Create/Remove/Rename of a .py file under an /api/ directory.
// It returns a stop function to tear the watcher down.
func Watch(root string, debounce time.Duration, onQualifying func(Event)) (func(), error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := addRecursive(w, root); err != nil {
		w.Close()
		return nil, err
	}

	f := newFilter(debounce)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if f.shouldTrigger(ev) {
					onQualifying(Event{Path: ev.Name, Op: ev.Op})
				}
			case <-w.Errors:
				// errors are surfaced to the caller only via onQualifying
				// being silent; a broken watcher will simply stop firing,
				// which the supervisor's own child-process health check
				// (Task 9) is responsible for noticing.
			case <-done:
				return
			}
		}
	}()

	return func() {
		close(done)
		w.Close()
	}, nil
}

func addRecursive(w *fsnotify.Watcher, root string) error {
	return walkDirs(root, func(dir string) error {
		return w.Add(dir)
	})
}

func walkDirs(root string, fn func(dir string) error) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") && path != root {
			return filepath.SkipDir
		}
		return fn(path)
	})
}
