package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/AlmostWorkingSystem/enigma-cli/internal/appgen"
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
	fmt.Fprintln(os.Stderr, "usage: enigma-cli <makeurls|server> [--root path] [addr]")
}

func runMakeURLs(args []string) {
	fs := flag.NewFlagSet("makeurls", flag.ExitOnError)
	root := fs.String("root", ".", "path to the Django project root")
	fs.Parse(args)

	elapsed, err := appgen.Generate(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("URL generation completed in %s.\n", elapsed)
}

func runServer(args []string) {
	fmt.Fprintln(os.Stderr, "server subcommand not yet implemented")
	os.Exit(1)
}
