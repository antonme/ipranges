package main

import (
	"fmt"
	"io"
	"os"
)

var subcommands = map[string]struct{}{
	"fetch":      {},
	"build":      {},
	"verify":     {},
	"query":      {},
	"compare":    {},
	"whois-shim": {},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	subcommand := args[0]
	if _, ok := subcommands[subcommand]; !ok {
		fmt.Fprintf(stderr, "unknown subcommand %q\n", subcommand)
		printUsage(stderr)
		return 2
	}

	fmt.Fprintf(stderr, "%s is not implemented yet\n", subcommand)
	return 1
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: irrindex <fetch|build|verify|query|compare|whois-shim> [options]")
}
