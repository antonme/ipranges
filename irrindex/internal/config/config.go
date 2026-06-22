package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type SourcesConfig struct {
	Sources []Source
}

type Source struct {
	Name        string   `yaml:"-"`
	URL         string   `yaml:"url"`
	Format      string   `yaml:"format"`
	ObjectTypes []string `yaml:"object_types"`
	Family      int      `yaml:"family"`
}

type QueriesConfig struct {
	Queries []Query `yaml:"queries"`
}

type Query struct {
	Downloader string   `yaml:"downloader"`
	Kind       string   `yaml:"kind"`
	Value      string   `yaml:"value"`
	Mirrors    []string `yaml:"mirrors"`
}

func LoadSources(path string) (SourcesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SourcesConfig{}, fmt.Errorf("read sources config: %w", err)
	}

	var raw struct {
		Sources orderedSources `yaml:"sources"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return SourcesConfig{}, fmt.Errorf("parse sources config: %w", err)
	}

	cfg := SourcesConfig{Sources: raw.Sources.Sources}
	for _, source := range cfg.Sources {
		if source.URL == "" {
			return SourcesConfig{}, fmt.Errorf("source %q has empty url", source.Name)
		}
		if source.Format == "" {
			return SourcesConfig{}, fmt.Errorf("source %q has empty format", source.Name)
		}
	}

	return cfg, nil
}

func LoadQueries(path string) (QueriesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return QueriesConfig{}, fmt.Errorf("read queries config: %w", err)
	}

	var cfg QueriesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return QueriesConfig{}, fmt.Errorf("parse queries config: %w", err)
	}

	for i, query := range cfg.Queries {
		if query.Downloader == "" {
			return QueriesConfig{}, fmt.Errorf("query %d has empty downloader", i)
		}
		if query.Value == "" {
			return QueriesConfig{}, fmt.Errorf("query %d has empty query value", i)
		}
		switch query.Kind {
		case "origin", "mnt-by":
		default:
			return QueriesConfig{}, fmt.Errorf("query %d has unsupported query kind %q", i, query.Kind)
		}
	}

	return cfg, nil
}

type orderedSources struct {
	Sources []Source
}

func (s *orderedSources) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == 0 {
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("sources must be a mapping")
	}
	if len(value.Content)%2 != 0 {
		return errors.New("sources mapping has odd node count")
	}

	seen := map[string]struct{}{}
	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i]
		body := value.Content[i+1]
		name := key.Value
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate source %q", name)
		}
		seen[name] = struct{}{}

		var source Source
		if err := body.Decode(&source); err != nil {
			return fmt.Errorf("decode source %q: %w", name, err)
		}
		source.Name = name
		s.Sources = append(s.Sources, source)
	}

	return nil
}
