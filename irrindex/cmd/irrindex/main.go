package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	indexcompare "github.com/antonme/ipranges/irrindex/internal/compare"
	"github.com/antonme/ipranges/irrindex/internal/config"
	"github.com/antonme/ipranges/irrindex/internal/fetch"
	"github.com/antonme/ipranges/irrindex/internal/manifest"
	"github.com/antonme/ipranges/irrindex/internal/mirrors"
	routequery "github.com/antonme/ipranges/irrindex/internal/query"
	"github.com/antonme/ipranges/irrindex/internal/ris"
	"github.com/antonme/ipranges/irrindex/internal/rpki"
	"github.com/antonme/ipranges/irrindex/internal/rpsl"
	"github.com/antonme/ipranges/irrindex/internal/store"
)

var subcommands = map[string]struct{}{
	"fetch":           {},
	"build":           {},
	"verify":          {},
	"query":           {},
	"compare":         {},
	"shadow-run":      {},
	"whois-shim":      {},
	"extract-queries": {},
	"manifest":        {},
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
	case "verify":
		return runVerify(args[1:], stdout, stderr)
	case "query":
		return runQuery(args[1:], stdout, stderr)
	case "whois-shim":
		return runWhoisShim(args[1:], stdout, stderr)
	case "extract-queries":
		return runExtractQueries(args[1:], stdout, stderr)
	case "compare":
		return runCompare(args[1:], stdout, stderr)
	case "shadow-run":
		return runShadowRun(args[1:], stdout, stderr)
	case "manifest":
		return runManifest(args[1:], stdout, stderr)
	}

	fmt.Fprintf(stderr, "%s is not implemented yet\n", subcommand)
	return 1
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: irrindex <fetch|build|verify|query|compare|shadow-run|whois-shim|extract-queries|manifest> [options]")
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

func runManifest(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: irrindex manifest summarize --manifest <snapshots.json>")
		return 2
	}
	switch args[0] {
	case "summarize":
		return runManifestSummarize(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown manifest subcommand %q\n", args[0])
		return 2
	}
}

func runManifestSummarize(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("manifest summarize", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", "", "snapshot manifest JSON")
	dbPath := flags.String("db", "", "optional SQLite DB path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *manifestPath == "" {
		fmt.Fprintln(stderr, "manifest summarize: --manifest is required")
		return 2
	}
	if *dbPath != "" {
		db, err := store.Open(*dbPath)
		if err != nil {
			fmt.Fprintf(stderr, "open db: %v\n", err)
			return 1
		}
		db.Close()
	}

	data, err := os.ReadFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "read manifest: %v\n", err)
		return 1
	}
	var snapshots []manifest.Snapshot
	if err := json.Unmarshal(data, &snapshots); err != nil {
		fmt.Fprintf(stderr, "parse manifest: %v\n", err)
		return 1
	}
	plan := manifest.PlanRetention(snapshots, manifest.DefaultRetentionPolicy())
	fmt.Fprintf(stdout, "keep=%d prune=%d\n", len(plan.Keep), len(plan.Prune))
	for _, snapshot := range plan.Keep {
		fmt.Fprintf(stdout, "keep %s %s %s\n", snapshot.Cadence, snapshot.CreatedAt.Format("2006-01-02"), snapshot.Name)
	}
	for _, snapshot := range plan.Prune {
		fmt.Fprintf(stdout, "prune %s %s %s\n", snapshot.Cadence, snapshot.CreatedAt.Format("2006-01-02"), snapshot.Name)
	}
	return 0
}

func runCompare(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoRoot := flags.String("repo", ".", "repository root")
	liveRoot := flags.String("live-root", "", "repository root alias")
	queriesPath := flags.String("queries", "irrindex/config/queries.yaml", "queries YAML path")
	dbPath := flags.String("db", "irrindex.db", "SQLite DB path")
	jsonPath := flags.String("json", "comparison.json", "JSON output path")
	mdPath := flags.String("md", "comparison.md", "Markdown output path")
	outDir := flags.String("out", "", "output directory for comparison reports")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *liveRoot != "" {
		*repoRoot = *liveRoot
	}
	if *outDir != "" {
		*jsonPath = filepath.Join(*outDir, "comparison.json")
		*mdPath = filepath.Join(*outDir, "comparison.md")
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

type downloaderFlag []string

func (f *downloaderFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *downloaderFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		downloader := strings.TrimSpace(part)
		if downloader == "" {
			continue
		}
		seen := false
		for _, existing := range *f {
			if existing == downloader {
				seen = true
				break
			}
		}
		if seen {
			continue
		}
		*f = append(*f, downloader)
	}
	return nil
}

type shadowRunOptions struct {
	repoRoot    string
	dbPath      string
	outDir      string
	irrindexBin string
	cacheDir    string
	reference   string
	downloaders []string
}

type shadowRunResult struct {
	workRoot string
	logPath  string
	jsonPath string
	mdPath   string
}

func runShadowRun(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("shadow-run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoRoot := flags.String("repo", ".", "repository root")
	dbPath := flags.String("db", "irrindex.db", "SQLite DB path")
	outDir := flags.String("out", "shadow-run-report", "output directory")
	irrindexBin := flags.String("irrindex-bin", "", "irrindex executable path")
	cacheDir := flags.String("cache-dir", "", "optional cached curl/whois response directory")
	reference := flags.String("reference", "committed", "comparison reference: committed or cached-whois")
	var downloaders downloaderFlag
	flags.Var(&downloaders, "downloader", "downloader name; may be repeated or comma-separated")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if len(downloaders) == 0 {
		fmt.Fprintln(stderr, "shadow-run: --downloader is required")
		return 2
	}

	result, err := runShadowDownloaders(shadowRunOptions{
		repoRoot:    *repoRoot,
		dbPath:      *dbPath,
		outDir:      *outDir,
		irrindexBin: *irrindexBin,
		cacheDir:    *cacheDir,
		reference:   *reference,
		downloaders: append([]string(nil), downloaders...),
	})
	if err != nil {
		fmt.Fprintf(stderr, "shadow-run: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "wrote %s and %s\n", result.jsonPath, result.mdPath)
	fmt.Fprintf(stdout, "workdir %s\n", result.workRoot)
	fmt.Fprintf(stdout, "log %s\n", result.logPath)
	return 0
}

func runShadowDownloaders(opts shadowRunOptions) (shadowRunResult, error) {
	repoRoot, err := filepath.Abs(opts.repoRoot)
	if err != nil {
		return shadowRunResult{}, fmt.Errorf("resolve repo root: %w", err)
	}
	dbPath, err := filepath.Abs(opts.dbPath)
	if err != nil {
		return shadowRunResult{}, fmt.Errorf("resolve db path: %w", err)
	}
	outDir, err := filepath.Abs(opts.outDir)
	if err != nil {
		return shadowRunResult{}, fmt.Errorf("resolve output dir: %w", err)
	}
	cacheDir := opts.cacheDir
	if cacheDir != "" {
		cacheDir, err = filepath.Abs(cacheDir)
		if err != nil {
			return shadowRunResult{}, fmt.Errorf("resolve cache dir: %w", err)
		}
	}
	irrindexBin := opts.irrindexBin
	if irrindexBin == "" {
		irrindexBin, err = os.Executable()
		if err != nil {
			return shadowRunResult{}, fmt.Errorf("resolve irrindex executable: %w", err)
		}
	}
	irrindexBin, err = filepath.Abs(irrindexBin)
	if err != nil {
		return shadowRunResult{}, fmt.Errorf("resolve irrindex executable path: %w", err)
	}

	for _, downloader := range opts.downloaders {
		if err := validateDownloaderName(downloader); err != nil {
			return shadowRunResult{}, err
		}
		if _, err := os.Stat(filepath.Join(repoRoot, downloader, "downloader.sh")); err != nil {
			return shadowRunResult{}, fmt.Errorf("%s: find downloader.sh: %w", downloader, err)
		}
	}
	reference := opts.reference
	if reference == "" {
		reference = "committed"
	}
	if reference != "committed" && reference != "cached-whois" {
		return shadowRunResult{}, fmt.Errorf("unsupported reference %q", reference)
	}
	if reference == "cached-whois" && cacheDir == "" {
		return shadowRunResult{}, fmt.Errorf("--reference cached-whois requires --cache-dir")
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return shadowRunResult{}, fmt.Errorf("create output dir: %w", err)
	}
	workRoot, err := os.MkdirTemp(outDir, "work-")
	if err != nil {
		return shadowRunResult{}, fmt.Errorf("create work dir: %w", err)
	}
	realWhois, _ := exec.LookPath("whois")
	realCurl, _ := exec.LookPath("curl")

	if err := prepareShadowWorkRoot(repoRoot, workRoot, opts.downloaders); err != nil {
		return shadowRunResult{}, err
	}
	referenceRoot := repoRoot
	if reference == "cached-whois" {
		referenceRoot, err = os.MkdirTemp(outDir, "reference-")
		if err != nil {
			return shadowRunResult{}, fmt.Errorf("create reference work dir: %w", err)
		}
		if err := prepareShadowWorkRoot(repoRoot, referenceRoot, opts.downloaders); err != nil {
			return shadowRunResult{}, err
		}
	}

	failureDir := ""
	if cacheDir != "" {
		failureDir, err = os.MkdirTemp(outDir, "failures-")
		if err != nil {
			return shadowRunResult{}, fmt.Errorf("create failure replay dir: %w", err)
		}
	}

	logPath := filepath.Join(outDir, "downloaders.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return shadowRunResult{}, fmt.Errorf("create downloader log: %w", err)
	}
	defer logFile.Close()

	if reference == "cached-whois" {
		if err := runShadowWorkRoot(referenceRoot, opts.downloaders, shadowRunEnv{
			dbPath:      dbPath,
			irrindexBin: irrindexBin,
			cacheDir:    cacheDir,
			realWhois:   realWhois,
			realCurl:    realCurl,
			failureDir:  failureDir,
			whoisMode:   "cache",
		}, logFile, logPath); err != nil {
			return shadowRunResult{}, err
		}
	}
	if err := runShadowWorkRoot(workRoot, opts.downloaders, shadowRunEnv{
		dbPath:      dbPath,
		irrindexBin: irrindexBin,
		cacheDir:    cacheDir,
		realWhois:   realWhois,
		realCurl:    realCurl,
		failureDir:  failureDir,
		whoisMode:   "local",
	}, logFile, logPath); err != nil {
		return shadowRunResult{}, err
	}

	report, err := indexcompare.BuildFileReport(referenceRoot, workRoot, opts.downloaders)
	if err != nil {
		return shadowRunResult{}, fmt.Errorf("compare shadow output: %w", err)
	}
	if reference == "cached-whois" {
		report.ComparisonScope = "local_whois_shadow_output_vs_cached_whois_shadow_output"
	}
	jsonPath := filepath.Join(outDir, "comparison.json")
	mdPath := filepath.Join(outDir, "comparison.md")
	if err := indexcompare.WriteJSON(jsonPath, report); err != nil {
		return shadowRunResult{}, err
	}
	if err := indexcompare.WriteMarkdown(mdPath, report); err != nil {
		return shadowRunResult{}, err
	}
	return shadowRunResult{workRoot: workRoot, logPath: logPath, jsonPath: jsonPath, mdPath: mdPath}, nil
}

type shadowRunEnv struct {
	dbPath      string
	irrindexBin string
	cacheDir    string
	realWhois   string
	realCurl    string
	failureDir  string
	whoisMode   string
}

func prepareShadowWorkRoot(repoRoot string, workRoot string, downloaders []string) error {
	shimDir := filepath.Join(workRoot, ".bin")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		return fmt.Errorf("create shim dir: %w", err)
	}
	if err := writeWhoisShim(filepath.Join(shimDir, "whois")); err != nil {
		return err
	}
	if err := writeCurlShim(filepath.Join(shimDir, "curl")); err != nil {
		return err
	}
	for _, tool := range []string{"dig", "nslookup"} {
		realPath, _ := exec.LookPath(tool)
		if err := writeReplayShim(filepath.Join(shimDir, tool), tool, realPath); err != nil {
			return err
		}
	}

	if err := copyDir(filepath.Join(repoRoot, "utils"), filepath.Join(workRoot, "utils")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("copy utils: %w", err)
	}
	for _, downloader := range downloaders {
		if err := copyDir(filepath.Join(repoRoot, downloader), filepath.Join(workRoot, downloader)); err != nil {
			return fmt.Errorf("copy %s: %w", downloader, err)
		}
	}
	return nil
}

func runShadowWorkRoot(workRoot string, downloaders []string, runEnv shadowRunEnv, logFile *os.File, logPath string) error {
	shimDir := filepath.Join(workRoot, ".bin")
	env := append(os.Environ(),
		"PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"IRRINDEX_BIN="+runEnv.irrindexBin,
		"IRRINDEX_DB="+runEnv.dbPath,
		"REAL_WHOIS="+runEnv.realWhois,
		"REAL_CURL="+runEnv.realCurl,
		"SHADOW_RESPONSE_CACHE_DIR="+runEnv.cacheDir,
		"SHADOW_RUN_FAILURE_DIR="+runEnv.failureDir,
		"SHADOW_WHOIS_MODE="+runEnv.whoisMode,
	)
	for _, downloader := range downloaders {
		cmd := exec.Command("bash", filepath.Join(downloader, "downloader.sh"))
		cmd.Dir = workRoot
		cmd.Env = env
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: run downloader: %w; see %s", downloader, err, logPath)
		}
	}
	return nil
}

func validateDownloaderName(name string) error {
	if name == "" || filepath.IsAbs(name) || strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid downloader name %q", name)
	}
	return nil
}

// shimCacheFuncs is shared by the shadow-run command shims. cached_run replays
// a recorded response, or runs the real command once and records the outcome:
// successes go to SHADOW_RESPONSE_CACHE_DIR, failures only to the per-invocation
// SHADOW_RUN_FAILURE_DIR, so the paired run replays the same failure without a
// transient outage poisoning later invocations.
const shimCacheFuncs = `cache_key() {
  printf '%s' "$1" | sed 's/[^A-Za-z0-9._-]/_/g'
}

cached_run() {
  kind="$1"
  key="$2"
  shift 2
  cache_file="${SHADOW_RESPONSE_CACHE_DIR}/${kind}/${key}"
  if [ -f "${cache_file}" ]; then
    cat "${cache_file}"
    return 0
  fi
  failed_file=""
  if [ -n "${SHADOW_RUN_FAILURE_DIR}" ]; then
    failed_file="${SHADOW_RUN_FAILURE_DIR}/${kind}/${key}"
    if [ -f "${failed_file}.status" ]; then
      cat "${failed_file}"
      return "$(cat "${failed_file}.status")"
    fi
  fi
  mkdir -p "${SHADOW_RESPONSE_CACHE_DIR}/${kind}"
  tmp="${cache_file}.tmp.$$"
  "$@" > "${tmp}"
  status="$?"
  if [ "${status}" = "0" ]; then
    mv "${tmp}" "${cache_file}"
    echo "shadow ${kind} cache miss recorded: ${key}" >&2
    cat "${cache_file}"
    return 0
  fi
  if [ -n "${failed_file}" ]; then
    mkdir -p "${SHADOW_RUN_FAILURE_DIR}/${kind}"
    cp "${tmp}" "${failed_file}"
    echo "${status}" > "${failed_file}.status"
    echo "shadow ${kind} failure recorded for paired run: ${key} (exit ${status})" >&2
  fi
  cat "${tmp}"
  rm -f "${tmp}"
  return "${status}"
}
`

func writeWhoisShim(path string) error {
	body := `#!/bin/sh
` + shimCacheFuncs + `
run_real_whois() {
  if [ -z "${REAL_WHOIS}" ]; then
    echo "no real whois available for: $*" >&2
    return 127
  fi
  timeout="${SHADOW_WHOIS_TIMEOUT:-30}"
  if [ "${timeout}" = "0" ]; then
    "${REAL_WHOIS}" "$@"
    return "$?"
  fi
  "${REAL_WHOIS}" "$@" &
  pid="$!"
  elapsed=0
  while kill -0 "${pid}" 2>/dev/null; do
    if [ "${elapsed}" -ge "${timeout}" ]; then
      kill "${pid}" 2>/dev/null || true
      wait "${pid}" 2>/dev/null
      return 124
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done
  wait "${pid}"
  return "$?"
}

host=""
if [ "$1" = "-h" ]; then
  host="$2"
  shift 2
  if [ "${SHADOW_WHOIS_MODE}" != "cache" ]; then
    case "$host" in
      riswhois.ripe.net|whois.radb.net|whois.ripe.net|rr.ntt.net|whois.rogerstelecom.net|whois.bgp.net.br)
        exec "${IRRINDEX_BIN}" whois-shim --db "${IRRINDEX_DB}" -h "$host" "$@"
        ;;
    esac
  fi
  if [ -n "${SHADOW_RESPONSE_CACHE_DIR}" ]; then
    cached_run whois "$(cache_key "${host}_$*")" run_real_whois -h "$host" "$@"
    exit "$?"
  fi
fi
if [ -n "${host}" ]; then
  run_real_whois -h "$host" "$@"
else
  run_real_whois "$@"
fi
exit "$?"
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		return fmt.Errorf("write whois shim: %w", err)
	}
	return nil
}

func writeCurlShim(path string) error {
	body := `#!/bin/sh
` + shimCacheFuncs + `
if [ -n "${SHADOW_RESPONSE_CACHE_DIR}" ]; then
  saw_url=0
  all_cached=1
  for arg in "$@"; do
    case "$arg" in
      http://*|https://*|ftp://*)
        saw_url=1
        key="$(cache_key "${arg}")"
        cache_file="${SHADOW_RESPONSE_CACHE_DIR}/curl/${key}"
        if [ ! -f "${cache_file}" ]; then
          all_cached=0
        fi
        ;;
    esac
  done
  if [ "${saw_url}" = "1" ] && [ "${all_cached}" = "1" ]; then
    for arg in "$@"; do
      case "$arg" in
        http://*|https://*|ftp://*)
          key="$(cache_key "${arg}")"
          cat "${SHADOW_RESPONSE_CACHE_DIR}/curl/${key}"
          ;;
      esac
    done
    exit 0
  fi
  # Single-URL fetches to stdout are recorded for the paired shadow run;
  # other invocations pass through.
  if [ "${saw_url}" = "1" ] && [ -n "${REAL_CURL}" ]; then
    url_count=0
    writes_file=0
    url=""
    for arg in "$@"; do
      case "$arg" in
        http://*|https://*|ftp://*)
          url_count=$((url_count + 1))
          url="$arg"
          ;;
        -o|-O|--output|--remote-name|-o*|--output=*)
          writes_file=1
          ;;
      esac
    done
    if [ "${url_count}" = "1" ] && [ "${writes_file}" = "0" ]; then
      cached_run curl "$(cache_key "${url}")" "${REAL_CURL}" "$@"
      exit "$?"
    fi
  fi
fi

if [ -n "${REAL_CURL}" ]; then
  exec "${REAL_CURL}" "$@"
fi
echo "no real curl available for: $*" >&2
exit 127
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		return fmt.Errorf("write curl shim: %w", err)
	}
	return nil
}

// shellQuote single-quotes a value for embedding in a generated sh script.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// writeReplayShim writes a shim that records and replays a DNS tool's output,
// keyed by its full argument list, so paired shadow runs see the same answers.
func writeReplayShim(path string, tool string, realPath string) error {
	body := `#!/bin/sh
` + shimCacheFuncs + `
real=` + shellQuote(realPath) + `
if [ -z "${real}" ]; then
  echo "no real ` + tool + ` available for: $*" >&2
  exit 127
fi
if [ -n "${SHADOW_RESPONSE_CACHE_DIR}" ]; then
  cached_run ` + tool + ` "$(cache_key "$*")" "${real}" "$@"
  exit "$?"
fi
exec "${real}" "$@"
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		return fmt.Errorf("write %s shim: %w", tool, err)
	}
	return nil
}

func copyDir(src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(path, target)
	})
}

func copyFile(src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func runFetch(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("fetch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sourcesPath := flags.String("sources", "irrindex/config/sources.yaml", "sources YAML path")
	configPath := flags.String("config", "", "sources YAML path alias")
	cacheDir := flags.String("cache", "snapshots/cache", "cache directory")
	cacheDirAlias := flags.String("cache-dir", "", "cache directory alias")
	manifestPath := flags.String("manifest", "", "source manifest JSON path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *configPath != "" {
		*sourcesPath = *configPath
	}
	if *cacheDirAlias != "" {
		*cacheDir = *cacheDirAlias
	}

	cfg, err := config.LoadSources(*sourcesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load sources: %v\n", err)
		return 1
	}
	manifest, err := fetch.FetchAll(cfg, *cacheDir)
	printFetchManifest(stdout, manifest)
	if *manifestPath != "" {
		if writeErr := writeJSON(*manifestPath, manifest); writeErr != nil {
			fmt.Fprintf(stderr, "write manifest: %v\n", writeErr)
			return 1
		}
	}
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
	configPath := flags.String("config", "", "sources YAML path alias")
	cacheDir := flags.String("cache", "snapshots/cache", "cache directory")
	cacheDirAlias := flags.String("cache-dir", "", "cache directory alias")
	dbPath := flags.String("db", "irrindex.db", "SQLite output path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *configPath != "" {
		*sourcesPath = *configPath
	}
	if *cacheDirAlias != "" {
		*cacheDir = *cacheDirAlias
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
		if shouldSkipBuildSource(source) {
			fmt.Fprintf(stdout, "%s skipped unsupported_object_types\n", source.Name)
			continue
		}
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

func shouldSkipBuildSource(source config.Source) bool {
	if source.Format != "rpsl" || len(source.ObjectTypes) == 0 {
		return false
	}
	for _, objectType := range source.ObjectTypes {
		switch objectType {
		case "route", "route6":
			return false
		}
	}
	return true
}

func runVerify(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dbPath := flags.String("db", "irrindex.db", "SQLite DB path")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return 1
	}
	defer db.Close()

	var sourceCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM sources WHERE ok = 1`).Scan(&sourceCount); err != nil {
		fmt.Fprintf(stderr, "verify sources: %v\n", err)
		return 1
	}
	var objectCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM objects`).Scan(&objectCount); err != nil {
		fmt.Fprintf(stderr, "verify objects: %v\n", err)
		return 1
	}
	var defaultCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM objects WHERE prefix IN ('0.0.0.0/0', '::/0')`).Scan(&defaultCount); err != nil {
		fmt.Fprintf(stderr, "verify defaults: %v\n", err)
		return 1
	}
	if sourceCount == 0 || objectCount == 0 {
		fmt.Fprintf(stderr, "verify failed: sources=%d objects=%d\n", sourceCount, objectCount)
		return 1
	}
	if defaultCount > 0 {
		fmt.Fprintf(stderr, "verify failed: default route objects=%d\n", defaultCount)
		return 1
	}
	fmt.Fprintf(stdout, "verify ok sources=%d objects=%d\n", sourceCount, objectCount)
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
	if strings.TrimSpace(host) == "" {
		return nil, true
	}
	return mirrors.SourcesForHost(host)
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
	case "rpki_csv":
		objects, err := rpki.ParseCSV(gz)
		if err != nil {
			return 0, fmt.Errorf("parse RPKI CSV: %w", err)
		}
		if err := db.InsertRPSLSource(source.Name, objects); err != nil {
			return 0, err
		}
		return len(objects), nil
	default:
		return 0, fmt.Errorf("unsupported source format %q", source.Format)
	}
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create JSON dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	return nil
}
