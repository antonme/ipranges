package fetch

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/antonme/ipranges/irrindex/internal/config"
)

const (
	retryCount     = 3
	requestTimeout = 60 * time.Second
)

type Manifest struct {
	Sources []SourceResult `json:"sources"`
}

type SourceResult struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

func FetchAll(cfg config.SourcesConfig, cacheDir string) (Manifest, error) {
	if len(cfg.Sources) == 0 {
		return Manifest{}, fmt.Errorf("no sources configured")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("create cache dir: %w", err)
	}

	client := &http.Client{Timeout: requestTimeout}
	manifest := Manifest{Sources: make([]SourceResult, 0, len(cfg.Sources))}
	successes := 0
	for _, source := range cfg.Sources {
		result := SourceResult{
			Name: source.Name,
			URL:  source.URL,
			Path: filepath.Join(cacheDir, source.Name+".gz"),
		}

		err := fetchOne(client, source.URL, result.Path)
		if err != nil {
			result.Error = err.Error()
			manifest.Sources = append(manifest.Sources, result)
			continue
		}

		sum, size, err := fileDigest(result.Path)
		if err != nil {
			result.Error = err.Error()
			manifest.Sources = append(manifest.Sources, result)
			continue
		}
		result.SHA256 = sum
		result.SizeBytes = size
		result.OK = true
		successes++
		manifest.Sources = append(manifest.Sources, result)
	}

	if successes == 0 {
		return manifest, fmt.Errorf("all sources failed")
	}
	return manifest, nil
}

func fetchOne(client *http.Client, url string, path string) error {
	var lastErr error
	for attempt := 0; attempt < retryCount; attempt++ {
		if err := fetchAttempt(client, url, path); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func fetchAttempt(client *http.Client, url string, path string) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("GET %s: status %s", url, resp.Status)
	}

	tmpPath := path + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create cache file: %w", err)
	}
	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write cache file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close cache file: %w", closeErr)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename cache file: %w", err)
	}
	return nil
}

func fileDigest(path string) (string, int64, error) {
	in, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open cached file: %w", err)
	}
	defer in.Close()

	hash := sha256.New()
	size, err := io.Copy(hash, in)
	if err != nil {
		return "", 0, fmt.Errorf("hash cached file: %w", err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), size, nil
}
