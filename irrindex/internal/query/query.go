package query

import (
	"fmt"
	"sort"
	"strings"

	"github.com/antonme/ipranges/irrindex/internal/store"
)

func ByOrigin(db *store.DB, asn string, family int, sources []string) ([]string, error) {
	return byAttr(db, "origin", normalizeASN(asn), family, sources)
}

func ByMaintainer(db *store.DB, mnt string, family int, sources []string) ([]string, error) {
	return byAttr(db, "mnt-by", strings.ToUpper(strings.TrimSpace(mnt)), family, sources)
}

func byAttr(db *store.DB, attrName string, attrValue string, family int, sources []string) ([]string, error) {
	if db == nil || db.SQL() == nil {
		return nil, fmt.Errorf("database is nil")
	}

	sqlQuery := `SELECT o.prefix
FROM objects o
JOIN attrs a ON a.object_id = o.id
JOIN sources s ON s.id = o.source_id
WHERE o.family = ?
  AND o.prefix IS NOT NULL
  AND o.prefix != ''
  AND s.ok = 1
  AND a.name = ?
  AND a.value = ?`
	args := []any{family, attrName, attrValue}

	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, source := range sources {
			placeholders[i] = "?"
			args = append(args, source)
		}
		sqlQuery += " AND s.name IN (" + strings.Join(placeholders, ",") + ")"
	}

	rows, err := db.SQL().Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", attrName, err)
	}
	defer rows.Close()

	seen := map[string]struct{}{}
	for rows.Next() {
		var prefix string
		if err := rows.Scan(&prefix); err != nil {
			return nil, fmt.Errorf("scan prefix: %w", err)
		}
		seen[prefix] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate prefixes: %w", err)
	}

	prefixes := make([]string, 0, len(seen))
	for prefix := range seen {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	return prefixes, nil
}

func normalizeASN(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && strings.EqualFold(value[:2], "AS") {
		return "AS" + value[2:]
	}
	return "AS" + value
}
