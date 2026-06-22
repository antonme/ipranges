package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	indexcompare "github.com/antonme/ipranges/irrindex/internal/compare"
	"github.com/antonme/ipranges/irrindex/internal/config"
	"github.com/antonme/ipranges/irrindex/internal/fetch"
	"github.com/antonme/ipranges/irrindex/internal/manifest"
	routequery "github.com/antonme/ipranges/irrindex/internal/query"
	"github.com/antonme/ipranges/irrindex/internal/ris"
	"github.com/antonme/ipranges/irrindex/internal/rpsl"
	"github.com/antonme/ipranges/irrindex/internal/store"
)

var subcommands = map[string]struct{}{
	"fetch":           {},
	"build":           {},
	"verify":          {},
	"query":           {},
	"compare":         {},
	"whois-shim":      {},
	"extract-queries": {},
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
	case "query":
		return runQuery(args[1:], stdout, stderr)
	case "whois-shim":
		return runWhoisShim(args[1:], stdout, stderr)
	case "extract-queries":
		return runExtractQueries(args[1:], stdout, stderr)
	case "compare":
		return runCompare(args[1:], stdout, stderr)
	}

	fmt.Fprintf(stderr, "%s is not implemented yet\n", subcommand)
	return 1
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: irrindex <fetch|build|verify|query|compare|whois-shim|extract-queries> [options]")
}

func runQuery(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "Usage: irrindex query <origin|mnt-by> <value> --db <path> --family <4|6>")
		return 2
	}

	kind := args[0]
	value := args[1]
	flags := flag.NewFlagSet("query", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dbPath := flags.String("db", "irrindex.db", "SQLite DB path")
	family := flags.Int("family", 4, "IP family")
	if err := flags.Parse(args[2:]); err != nil {
		return 2
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return 1
	}
	defer db.Close()

	prefixes, err := queryPrefixes(db, kind, value, *family, nil)
	if err != nil {
		fmt.Fprintf(stderr, "query: %v\n", err)
		return 1
	}
	printCIDRs(stdout, prefixes)
	return 0
}

func runWhoisShim(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("whois-shim", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dbPath := flags.String("db", "irrindex.db", "SQLite DB path")
	host := flags.String("h", "", "whois host")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	kind, value, err := parseWhoisQuery(flags.Args())
	if err != nil {
		fmt.Fprintf(stderr, "parse whois query: %v\n", err)
		return 2
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return 1
	}
	defer db.Close()

	sources, knownHost := sourceFiltersForHost(*host)
	if *host != "" && !knownHost {
		fmt.Fprintf(stderr, "warning: unknown whois host %q; querying all sources\n", *host)
	}

	for _, family := range []int{4, 6} {
		prefixes, err := queryPrefixes(db, kind, value, family, sources)
		if err != nil {
			fmt.Fprintf(stderr, "query: %v\n", err)
			return 1
		}
		printWhoisRoutes(stdout, prefixes)
	}
	return 0
}

func runExtractQueries(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("extract-queries", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoRoot := flags.String("repo", ".", "repository root")
	outPath := flags.String("out", "irrindex/config/queries.yaml", "output queries YAML")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	specs, err := manifest.ExtractQueries(*repoRoot)
	if err != nil {
		fmt.Fprintf(stderr, "extract queries: %v\n", err)
		return 1
	}
	if err := manifest.WriteQueriesYAML(*outPath, specs); err != nil {
		fmt.Fprintf(stderr, "write queries: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %d queries to %s\n", len(specs), *outPath)
	return 0
}

func runCompare(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoRoot := flags.String("repo", ".", "repository root")
	queriesPath := flags.String("queries", "irrindex/config/queries.yaml", "queries YAML path")
	dbPath := flags.String("db", "irrindex.db", "SQLite DB path")
	jsonPath := flags.String("json", "comparison.json", "JSON output path")
	mdPath := flags.String("md", "comparison.md", "Markdown output path")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	queriesCfg, err := config.LoadQueries(*queriesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load queries: %v\n", err)
		return 1
	}
	db, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return 1
	}
	defer db.Close()

	report, err := indexcompare.BuildReport(*repoRoot, queriesCfg.Queries, db)
	if err != nil {
		fmt.Fprintf(stderr, "compare: %v\n", err)
		return 1
	}
	if err := indexcompare.WriteJSON(*jsonPath, report); err != nil {
		fmt.Fprintf(stderr, "write json: %v\n", err)
		return 1
	}
	if err := indexcompare.WriteMarkdown(*mdPath, report); err != nil {
		fmt.Fprintf(stderr, "write markdown: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s and %s\n", *jsonPath, *mdPath)
	return 0
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

func queryPrefixes(db *store.DB, kind string, value string, family int, sources []string) ([]string, error) {
	switch kind {
	case "origin":
		return routequery.ByOrigin(db, value, family, sources)
	case "mnt-by":
		return routequery.ByMaintainer(db, value, family, sources)
	default:
		return nil, fmt.Errorf("unsupported query kind %q", kind)
	}
}

func printCIDRs(stdout io.Writer, prefixes []string) {
	for _, prefix := range prefixes {
		fmt.Fprintln(stdout, prefix)
	}
}

func printWhoisRoutes(stdout io.Writer, prefixes []string) {
	for _, prefix := range prefixes {
		label := "route:"
		if strings.Contains(prefix, ":") {
			label = "route6:"
		}
		fmt.Fprintf(stdout, "%-16s%s\n", label, prefix)
	}
}

func parseWhoisQuery(args []string) (string, string, error) {
	fields := strings.Fields(strings.Join(args, " "))
	for i := 0; i+2 < len(fields); i++ {
		if fields[i] != "-i" {
			continue
		}
		kind := fields[i+1]
		value := fields[i+2]
		if kind != "origin" && kind != "mnt-by" {
			return "", "", fmt.Errorf("unsupported inverse query %q", kind)
		}
		return kind, value, nil
	}
	return "", "", fmt.Errorf("expected -i <origin|mnt-by> <value>")
}

func sourceFiltersForHost(host string) ([]string, bool) {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "":
		return nil, true
	case "whois.radb.net":
		return []string{"radb"}, true
	case "whois.ripe.net":
		return []string{"ripe", "ripe_route", "ripe_route6", "ripe_inetnum", "ripe_inet6num"}, true
	case "riswhois.ripe.net":
		return []string{"ris", "ris_ipv4", "ris_ipv6"}, true
	case "rr.ntt.net":
		return []string{"ntt", "nttcom", "nttcom_route", "nttcom_route6"}, true
	case "whois.rogerstelecom.net":
		return []string{"rogers", "rogerstelecom"}, true
	case "whois.bgp.net.br":
		return []string{"bgp_net_br", "bgp.net.br"}, true
	default:
		return nil, false
	}
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
