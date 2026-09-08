package watch

import (
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestShouldTrigger_CreateUnderAPIDir(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	if !f.shouldTrigger(fsnotify.Event{Name: "modules/demo/api/v1/widget.py", Op: fsnotify.Create}) {
		t.Fatalf("want create under /api/ to trigger")
	}
}

func TestShouldTrigger_IgnoresPlainWrite(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	if f.shouldTrigger(fsnotify.Event{Name: "modules/demo/api/v1/widget.py", Op: fsnotify.Write}) {
		t.Fatalf("plain write must not trigger")
	}
}

func TestShouldTrigger_IgnoresNonAPIPath(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	if f.shouldTrigger(fsnotify.Event{Name: "modules/demo/services/widget.py", Op: fsnotify.Create}) {
		t.Fatalf("create outside /api/ must not trigger")
	}
}

func TestShouldTrigger_IgnoresRoutesFile(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	if f.shouldTrigger(fsnotify.Event{Name: "_routes.py", Op: fsnotify.Create}) {
		t.Fatalf("_routes.py must never trigger (would self-loop)")
	}
}

func TestShouldTrigger_IgnoresNonPyFiles(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	if f.shouldTrigger(fsnotify.Event{Name: "modules/demo/api/v1/notes.txt", Op: fsnotify.Create}) {
		t.Fatalf("non-.py files must not trigger")
	}
}

func TestShouldTrigger_DebouncesRepeatedEvents(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	ev := fsnotify.Event{Name: "modules/demo/api/v1/widget.py", Op: fsnotify.Create}
	if !f.shouldTrigger(ev) {
		t.Fatalf("first event should trigger")
	}
	if f.shouldTrigger(ev) {
		t.Fatalf("immediate repeat should be debounced")
	}
}
