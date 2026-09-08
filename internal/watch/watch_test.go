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

func TestShouldTrigger_AcceptsPlainWrite(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	if !f.shouldTrigger(fsnotify.Event{Name: "modules/demo/api/v1/widget.py", Op: fsnotify.Write}) {
		t.Fatalf("plain write of a qualifying .py path must trigger (restart-worthy, even though not API-regen-worthy)")
	}
}

func TestShouldTrigger_AcceptsWriteOutsideAPIDir(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	if !f.shouldTrigger(fsnotify.Event{Name: "modules/demo/services/widget.py", Op: fsnotify.Write}) {
		t.Fatalf("write outside /api/ must still trigger a restart — the debounced gate is not scoped to /api/")
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
		t.Fatalf("non-.py/.html/.env files must not trigger")
	}
}

func TestShouldTrigger_AcceptsHTMLFiles(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	if !f.shouldTrigger(fsnotify.Event{Name: "templates/email/welcome.html", Op: fsnotify.Write}) {
		t.Fatalf(".html files must trigger, matching Python's patterns=[\"*.py\", \"*.html\", \".env\"]")
	}
}

func TestShouldTrigger_AcceptsDotEnv(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	if !f.shouldTrigger(fsnotify.Event{Name: ".env", Op: fsnotify.Write}) {
		t.Fatalf(".env must trigger, matching Python's patterns=[\"*.py\", \"*.html\", \".env\"]")
	}
}

func TestShouldTrigger_IgnoresDotEnvSuffixMatch(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	if f.shouldTrigger(fsnotify.Event{Name: "config/foo.env", Op: fsnotify.Write}) {
		t.Fatalf("only the exact basename \".env\" should trigger, not any *.env suffix")
	}
}

func TestShouldTrigger_IgnoresPycFiles(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	if f.shouldTrigger(fsnotify.Event{Name: "modules/demo/api/v1/widget.pyc", Op: fsnotify.Create}) {
		t.Fatalf(".pyc files must not trigger, matching Python's ignore_patterns=[\"*.pyc\", ...]")
	}
}

func TestShouldTrigger_IgnoresPycache(t *testing.T) {
	f := newFilter(500 * time.Millisecond)
	if f.shouldTrigger(fsnotify.Event{Name: "modules/demo/__pycache__/widget.py", Op: fsnotify.Create}) {
		t.Fatalf("paths under __pycache__ must not trigger, matching Python's ignore_patterns=[\"__pycache__/*\", ...]")
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

func TestIsAPIFileChange_CreateUnderAPIDir(t *testing.T) {
	if !IsAPIFileChange(Event{Path: "modules/demo/api/v1/widget.py", Op: fsnotify.Create}) {
		t.Fatalf("want create of .py under /api/ to be an API file change")
	}
}

func TestIsAPIFileChange_RemoveUnderAPIDir(t *testing.T) {
	if !IsAPIFileChange(Event{Path: "modules/demo/api/v1/widget.py", Op: fsnotify.Remove}) {
		t.Fatalf("want remove of .py under /api/ to be an API file change")
	}
}

func TestIsAPIFileChange_RenameUnderAPIDir(t *testing.T) {
	if !IsAPIFileChange(Event{Path: "modules/demo/api/v1/widget.py", Op: fsnotify.Rename}) {
		t.Fatalf("want rename of .py under /api/ to be an API file change")
	}
}

func TestIsAPIFileChange_FalseForWrite(t *testing.T) {
	if IsAPIFileChange(Event{Path: "modules/demo/api/v1/widget.py", Op: fsnotify.Write}) {
		t.Fatalf("plain write must never be an API file change, even under /api/")
	}
}

func TestIsAPIFileChange_FalseForNonAPIPath(t *testing.T) {
	if IsAPIFileChange(Event{Path: "modules/demo/services/widget.py", Op: fsnotify.Create}) {
		t.Fatalf("create outside /api/ must not be an API file change")
	}
}

func TestIsAPIFileChange_FalseForHTMLUnderAPIDir(t *testing.T) {
	if IsAPIFileChange(Event{Path: "modules/demo/api/v1/widget.html", Op: fsnotify.Create}) {
		t.Fatalf("only .py files qualify as API file changes, not .html")
	}
}
