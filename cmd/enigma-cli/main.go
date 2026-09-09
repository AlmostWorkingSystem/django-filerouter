package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/AlmostWorkingSystem/enigma-cli/internal/apitemplate"
	"github.com/AlmostWorkingSystem/enigma-cli/internal/appgen"
	"github.com/AlmostWorkingSystem/enigma-cli/internal/routegen"
	"github.com/AlmostWorkingSystem/enigma-cli/internal/supervisor"
	"github.com/AlmostWorkingSystem/enigma-cli/internal/watch"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "makeurls":
		runMakeURLs(os.Args[2:])
	case "server":
		runServer(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: enigma-cli <makeurls|server> [--root path] [--dev] [addr]")
}

// resolveMode returns Eager unless --dev is set. Eager is the production
// format: every route resolves (and every view module imports) once, at
// _routes.py import time, so nothing needs to happen again per-request or
// for schema generation — favors fast, consistent request serving over
// server boot time. --dev switches to Lazy, which defers each route's
// view-module import until it's actually needed (a real request, or schema
// introspection) — much faster server boot, at the cost of a one-time
// per-route import on first use instead of at startup. Use --dev for local
// iteration, leave it off in production.
func resolveMode(dev bool) routegen.Mode {
	if dev {
		return routegen.Lazy
	}
	return routegen.Eager
}

func runMakeURLs(args []string) {
	fs := flag.NewFlagSet("makeurls", flag.ExitOnError)
	root := fs.String("root", ".", "path to the Django project root")
	dev := fs.Bool("dev", false, "generate the dev-loop (lazy-loading) route format instead of the production format")
	fs.Parse(args)

	elapsed, err := appgen.Generate(*root, resolveMode(*dev))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("URL generation completed in %s.\n", elapsed)
}

func runServer(args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	root := fs.String("root", ".", "path to the Django project root")
	dev := fs.Bool("dev", false, "generate the dev-loop (lazy-loading) route format instead of the production format")
	fs.Parse(args)
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: enigma-cli server [--root path] [--dev] <addr>")
		os.Exit(1)
	}
	addr := fs.Arg(0)
	mode := resolveMode(*dev)

	elapsed, err := appgen.Generate(*root, mode)
	if err != nil {
		fmt.Fprintln(os.Stderr, "initial makeurls failed:", err)
		os.Exit(1)
	}
	fmt.Printf("URL generation completed in %s.\n", elapsed)

	sup := supervisor.New(*root, "./manage.py", "runsslserver", addr)
	if err := sup.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "starting runsslserver failed:", err)
		os.Exit(1)
	}

	stopWatch, err := watch.Watch(*root, 500*time.Millisecond, func(ev watch.Event) {
		// IsAPIFileChange gates the narrower "regenerate routes" behavior;
		// every qualifying event reaching this callback restarts the
		// supervised server regardless, matching the real Python dev
		// server's unconditional process.terminate()/start() at the end of
		// on_any_event.
		if watch.IsAPIFileChange(ev) {
			if ev.Op&fsnotify.Create != 0 {
				maybeScaffold(ev.Path)
			}
			fmt.Println("Regenerating URLs due to file change:", ev.Path)
			if _, err := appgen.Generate(*root, mode); err != nil {
				fmt.Fprintln(os.Stderr, "makeurls failed, keeping old _routes.py:", err)
			}
		} else {
			fmt.Println("Restarting server due to file change:", ev.Path)
		}
		if err := sup.Restart(); err != nil {
			fmt.Fprintln(os.Stderr, "restart failed:", err)
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "starting watcher failed:", err)
		sup.Stop()
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	stopWatch()
	sup.Stop()
}

func maybeScaffold(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() != 0 {
		return
	}
	content := apitemplate.Render(path)
	_ = os.WriteFile(path, []byte(content), 0o644)
}
