package rpki

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/netip"
	"strings"

	"github.com/antonme/ipranges/irrindex/internal/rpsl"
)

func ParseCSV(r io.Reader) ([]rpsl.Object, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read RPKI CSV header: %w", err)
	}
	columns := columnIndexes(header)
	required := []string{"ASN", "IP Prefix", "Max Length", "Trust Anchor", "Expires"}
	for _, name := range required {
		if _, ok := columns[name]; !ok {
			return nil, fmt.Errorf("RPKI CSV missing column %q", name)
		}
	}

	var objects []rpsl.Object
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read RPKI CSV row: %w", err)
		}
		object, err := parseRow(row, columns)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	return objects, nil
}

func columnIndexes(header []string) map[string]int {
	columns := map[string]int{}
	for i, name := range header {
		columns[strings.TrimSpace(name)] = i
	}
	return columns
}

func parseRow(row []string, columns map[string]int) (rpsl.Object, error) {
	value := func(name string) string {
		index := columns[name]
		if index >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[index])
	}

	prefix, err := netip.ParsePrefix(value("IP Prefix"))
	if err != nil {
		return rpsl.Object{}, fmt.Errorf("parse RPKI prefix %q: %w", value("IP Prefix"), err)
	}
	objectType := "route6"
	if prefix.Addr().Is4() {
		objectType = "route"
	}

	return rpsl.Object{
		Type:   objectType,
		Prefix: prefix.Masked().String(),
		Attrs: map[string][]string{
			"origin":       {normalizeASN(value("ASN"))},
			"max-length":   {value("Max Length")},
			"trust-anchor": {value("Trust Anchor")},
			"expires":      {value("Expires")},
		},
	}, nil
}

func normalizeASN(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "AS") {
		return upper
	}
	return "AS" + upper
}
