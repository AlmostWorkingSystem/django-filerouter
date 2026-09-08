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
// PatternMatchingEventHandler pattern match (modules/core/management/commands/server.py:99-105):
// patterns=["*.py", "*.html", ".env"], ignore_patterns=["*.pyc", "__pycache__/*", "_routes.py"].
// This is the single "does this event even get considered" gate — it
// accepts ALL fsnotify ops (Create, Write, Remove, Rename), matching
// Python's on_any_event being invoked for every non-ignored, non-debounced
// event regardless of type. It does NOT decide whether to regenerate
// makeurls (see IsAPIFileChange) or whether to restart (every event that
// passes this gate restarts, per Python's unconditional
// self.process.terminate(); self.start() at the end of on_any_event).
type filter struct {
	debounce time.Duration
	mu       sync.Mutex
	last     map[string]time.Time
}

func newFilter(debounce time.Duration) *filter {
	return &filter{debounce: debounce, last: map[string]time.Time{}}
}

// isQualifyingPath implements the PatternMatchingEventHandler patterns
// Python's WatchDogReloader is constructed with: never _routes.py, never a
// path under __pycache__, and a suffix of .py or .html, or the exact
// basename ".env" (patterns=["*.py", "*.html", ".env"],
// ignore_patterns=["*.pyc", "__pycache__/*", "_routes.py"]).
func isQualifyingPath(name string) bool {
	base := filepath.Base(name)
	if base == "_routes.py" {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if part == "__pycache__" {
			return false
		}
	}
	if base == ".env" {
		return true
	}
	return strings.HasSuffix(name, ".py") || strings.HasSuffix(name, ".html")
}

func (f *filter) shouldTrigger(ev fsnotify.Event) bool {
	if !isQualifyingPath(ev.Name) {
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

// IsAPIFileChange reports whether ev should trigger a makeurls regeneration
// (and API-file scaffold), matching Python's narrower "regenerate" gate
// (modules/core/management/commands/server.py:65-69):
//
//	not event.event_type == EVENT_TYPE_MODIFIED and src_path.endswith(".py") and "/api/" in src_path
//
// i.e. Create/Remove/Rename (never plain Write) of a .py file whose path
// contains "/api/". This is strictly narrower than the debounced gate that
// decides whether onQualifying fires at all (filter.shouldTrigger) — every
// event reaching onQualifying should restart the server, but only events
// satisfying IsAPIFileChange should also regenerate routes.
func IsAPIFileChange(ev Event) bool {
	if ev.Op == fsnotify.Write {
		return false
	}
	return strings.HasSuffix(ev.Path, ".py") && strings.Contains(ev.Path, "/api/")
}

// Watch recursively watches root and calls onQualifying for every
// debounced filesystem event matching the *.py/*.html/.env patterns
// (excluding _routes.py and __pycache__), regardless of op type. Use
// IsAPIFileChange on the resulting Event to decide whether it also
// warrants a makeurls regeneration.
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
