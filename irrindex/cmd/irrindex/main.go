package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/antonme/ipranges/irrindex/internal/config"
	"github.com/antonme/ipranges/irrindex/internal/fetch"
	"github.com/antonme/ipranges/irrindex/internal/ris"
	"github.com/antonme/ipranges/irrindex/internal/rpsl"
	"github.com/antonme/ipranges/irrindex/internal/store"
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

	switch subcommand {
	case "fetch":
		return runFetch(args[1:], stdout, stderr)
	case "build":
		return runBuild(args[1:], stdout, stderr)
	}

	fmt.Fprintf(stderr, "%s is not implemented yet\n", subcommand)
	return 1
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: irrindex <fetch|build|verify|query|compare|whois-shim> [options]")
}

func runFetch(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("fetch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sourcesPath := flags.String("sources", "irrindex/config/sources.yaml", "sources YAML path")
	cacheDir := flags.String("cache", "snapshots/cache", "cache directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.LoadSources(*sourcesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load sources: %v\n", err)
		return 1
	}
	manifest, err := fetch.FetchAll(cfg, *cacheDir)
	printFetchManifest(stdout, manifest)
	if err != nil {
		fmt.Fprintf(stderr, "fetch: %v\n", err)
		return 1
	}
	return 0
}

func printFetchManifest(stdout io.Writer, manifest fetch.Manifest) {
	for _, source := range manifest.Sources {
		if source.OK {
			fmt.Fprintf(stdout, "%s ok %s %s %d\n", source.Name, source.Path, source.SHA256, source.SizeBytes)
			continue
		}
		fmt.Fprintf(stdout, "%s error %s\n", source.Name, source.Error)
	}
}

func runBuild(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sourcesPath := flags.String("sources", "irrindex/config/sources.yaml", "sources YAML path")
	cacheDir := flags.String("cache", "snapshots/cache", "cache directory")
	dbPath := flags.String("db", "irrindex.db", "SQLite output path")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.LoadSources(*sourcesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load sources: %v\n", err)
		return 1
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return 1
	}
	defer db.Close()

	successes := 0
	for _, source := range cfg.Sources {
		count, err := buildSource(db, source, filepath.Join(*cacheDir, source.Name+".gz"))
		if err != nil {
			if recordErr := db.RecordSourceError(source.Name, err); recordErr != nil {
				fmt.Fprintf(stderr, "record %s error: %v\n", source.Name, recordErr)
			}
			fmt.Fprintf(stdout, "%s error %v\n", source.Name, err)
			continue
		}
		successes++
		fmt.Fprintf(stdout, "%s ok %d\n", source.Name, count)
	}
	if successes == 0 {
		fmt.Fprintln(stderr, "build: all sources failed")
		return 1
	}
	return 0
}

func buildSource(db *store.DB, source config.Source, path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open cached source: %w", err)
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return 0, fmt.Errorf("open gzip source: %w", err)
	}
	defer gz.Close()

	switch source.Format {
	case "rpsl":
		objects, err := rpsl.Parse(gz)
		if err != nil {
			return 0, fmt.Errorf("parse RPSL: %w", err)
		}
		if err := db.InsertRPSLSource(source.Name, objects); err != nil {
			return 0, err
		}
		return len(objects), nil
	case "riswhois":
		records, err := ris.Parse(gz)
		if err != nil {
			return 0, fmt.Errorf("parse RISWhois: %w", err)
		}
		if err := db.InsertRISSource(source.Name, records, source.Family); err != nil {
			return 0, err
		}
		return len(records), nil
	default:
		return 0, fmt.Errorf("unsupported source format %q", source.Format)
	}
}
