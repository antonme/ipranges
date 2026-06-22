package store

import (
	"database/sql"
	_ "embed"
	"encoding/binary"
	"fmt"
	"math/bits"
	"net/netip"
	"strings"

	"github.com/antonme/ipranges/irrindex/internal/ris"
	"github.com/antonme/ipranges/irrindex/internal/rpsl"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type DB struct {
	conn            *sql.DB
	droppedDefaults int
}

type Stats struct {
	DroppedDefaults int
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	if _, err := conn.Exec(schemaSQL); err != nil {
		conn.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	if db == nil || db.conn == nil {
		return nil
	}
	return db.conn.Close()
}

func (db *DB) SQL() *sql.DB {
	return db.conn
}

func (db *DB) Stats() Stats {
	return Stats{DroppedDefaults: db.droppedDefaults}
}

func (db *DB) InsertRPSLSource(sourceName string, objects []rpsl.Object) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin insert RPSL source: %w", err)
	}
	defer tx.Rollback()

	sourceID, err := upsertSource(tx, sourceName)
	if err != nil {
		return err
	}
	if err := replaceSourceObjects(tx, sourceID); err != nil {
		return err
	}

	objectStmt, err := tx.Prepare(`INSERT INTO objects(source_id, object_type, family, prefix, start_ip, end_ip) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare object insert: %w", err)
	}
	defer objectStmt.Close()

	attrStmt, err := tx.Prepare(`INSERT INTO attrs(object_id, name, value) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare attr insert: %w", err)
	}
	defer attrStmt.Close()

	for _, obj := range objects {
		rows, ok, err := rpslObjectRows(obj)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		for _, row := range rows {
			if isDefaultPrefix(row.prefix) {
				db.droppedDefaults++
				continue
			}

			res, err := objectStmt.Exec(sourceID, row.objectType, row.family, nullableString(row.prefix), nullableString(row.startIP), nullableString(row.endIP))
			if err != nil {
				return fmt.Errorf("insert RPSL object: %w", err)
			}
			objectID, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("get RPSL object id: %w", err)
			}

			if err := insertAttrs(attrStmt, objectID, obj.Attrs); err != nil {
				return err
			}
		}
	}

	err = tx.Commit()
	return err
}

func (db *DB) InsertRISSource(sourceName string, records []ris.Record, family int) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin insert RIS source: %w", err)
	}
	defer tx.Rollback()

	sourceID, err := upsertSource(tx, sourceName)
	if err != nil {
		return err
	}
	if err := replaceSourceObjects(tx, sourceID); err != nil {
		return err
	}

	objectStmt, err := tx.Prepare(`INSERT INTO objects(source_id, object_type, family, prefix, peer_count) VALUES (?, 'ris', ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare RIS object insert: %w", err)
	}
	defer objectStmt.Close()

	attrStmt, err := tx.Prepare(`INSERT INTO attrs(object_id, name, value) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare attr insert: %w", err)
	}
	defer attrStmt.Close()

	for _, record := range records {
		prefix, parsedFamily, err := normalizePrefix(record.Prefix)
		if err != nil {
			return fmt.Errorf("normalize RIS prefix %q: %w", record.Prefix, err)
		}
		if family != 0 && parsedFamily != family {
			return fmt.Errorf("RIS prefix %q family = %d, want %d", record.Prefix, parsedFamily, family)
		}
		if record.IsDefault || isDefaultPrefix(prefix) {
			db.droppedDefaults++
			continue
		}

		res, err := objectStmt.Exec(sourceID, parsedFamily, prefix, record.PeerCount)
		if err != nil {
			return fmt.Errorf("insert RIS object: %w", err)
		}
		objectID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("get RIS object id: %w", err)
		}
		if err := insertAttr(attrStmt, objectID, "origin", normalizeASN(record.ASN)); err != nil {
			return err
		}
	}

	err = tx.Commit()
	return err
}

type objectRow struct {
	objectType string
	family     int
	prefix     string
	startIP    string
	endIP      string
}

func rpslObjectRows(obj rpsl.Object) ([]objectRow, bool, error) {
	objectType := strings.ToLower(strings.TrimSpace(obj.Type))
	row := objectRow{objectType: objectType}

	switch objectType {
	case "route", "route6", "inet6num":
		prefix, family, err := normalizePrefix(obj.Prefix)
		if err != nil {
			return nil, false, fmt.Errorf("normalize %s prefix %q: %w", objectType, obj.Prefix, err)
		}
		row.family = family
		row.prefix = prefix
		return []objectRow{row}, true, nil
	case "inetnum":
		row.family = 4
		row.startIP = strings.TrimSpace(obj.StartIP)
		row.endIP = strings.TrimSpace(obj.EndIP)
		if obj.Prefix != "" {
			prefix, family, err := normalizePrefix(obj.Prefix)
			if err != nil {
				return nil, false, fmt.Errorf("normalize inetnum prefix %q: %w", obj.Prefix, err)
			}
			row.family = family
			row.prefix = prefix
			return []objectRow{row}, true, nil
		}
		prefixes, err := ipv4RangeToPrefixes(row.startIP, row.endIP)
		if err != nil {
			return nil, false, fmt.Errorf("convert inetnum range %q - %q: %w", row.startIP, row.endIP, err)
		}
		rows := make([]objectRow, 0, len(prefixes))
		for _, prefix := range prefixes {
			next := row
			next.prefix = prefix
			rows = append(rows, next)
		}
		return rows, true, nil
	default:
		return nil, false, nil
	}
}

func upsertSource(tx *sql.Tx, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("source name is empty")
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO sources(name) VALUES (?)`, name); err != nil {
		return 0, fmt.Errorf("insert source %q: %w", name, err)
	}

	var sourceID int64
	if err := tx.QueryRow(`SELECT id FROM sources WHERE name = ?`, name).Scan(&sourceID); err != nil {
		return 0, fmt.Errorf("select source %q: %w", name, err)
	}
	return sourceID, nil
}

func replaceSourceObjects(tx *sql.Tx, sourceID int64) error {
	if _, err := tx.Exec(`DELETE FROM objects WHERE source_id = ?`, sourceID); err != nil {
		return fmt.Errorf("delete old source objects: %w", err)
	}
	return nil
}

func insertAttrs(stmt *sql.Stmt, objectID int64, attrs map[string][]string) error {
	for name, values := range attrs {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		for _, value := range values {
			if err := insertAttr(stmt, objectID, name, normalizeAttrValue(name, value)); err != nil {
				return err
			}
		}
	}
	return nil
}

func insertAttr(stmt *sql.Stmt, objectID int64, name string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if _, err := stmt.Exec(objectID, name, value); err != nil {
		return fmt.Errorf("insert attr %s: %w", name, err)
	}
	return nil
}

func normalizeAttrValue(name string, value string) string {
	switch name {
	case "origin":
		return normalizeASN(value)
	case "mnt-by":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return strings.TrimSpace(value)
	}
}

func normalizeASN(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && strings.EqualFold(value[:2], "AS") {
		return "AS" + value[2:]
	}
	return "AS" + value
}

func normalizePrefix(value string) (string, int, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
	if err != nil {
		return "", 0, err
	}
	prefix = prefix.Masked()
	if prefix.Addr().Is4() {
		return prefix.String(), 4, nil
	}
	return prefix.String(), 6, nil
}

func ipv4RangeToPrefixes(startValue string, endValue string) ([]string, error) {
	startAddr, err := netip.ParseAddr(strings.TrimSpace(startValue))
	if err != nil {
		return nil, fmt.Errorf("parse start IP: %w", err)
	}
	endAddr, err := netip.ParseAddr(strings.TrimSpace(endValue))
	if err != nil {
		return nil, fmt.Errorf("parse end IP: %w", err)
	}
	if !startAddr.Is4() || !endAddr.Is4() {
		return nil, fmt.Errorf("only IPv4 inetnum ranges are supported")
	}

	start := ipv4ToUint32(startAddr)
	end := ipv4ToUint32(endAddr)
	if start > end {
		return nil, fmt.Errorf("start IP is greater than end IP")
	}

	var prefixes []string
	current := uint64(start)
	last := uint64(end)
	for current <= last {
		blockSize := largestAlignedBlock(current)
		remaining := last - current + 1
		for blockSize > remaining {
			blockSize >>= 1
		}

		prefixLen := 32 - (bits.Len64(blockSize) - 1)
		prefixes = append(prefixes, fmt.Sprintf("%s/%d", uint32ToIPv4(uint32(current)), prefixLen))
		current += blockSize
	}
	return prefixes, nil
}

func largestAlignedBlock(value uint64) uint64 {
	if value == 0 {
		return uint64(1) << 32
	}
	return value & -value
}

func ipv4ToUint32(addr netip.Addr) uint32 {
	bytes := addr.As4()
	return binary.BigEndian.Uint32(bytes[:])
}

func uint32ToIPv4(value uint32) netip.Addr {
	var bytes [4]byte
	binary.BigEndian.PutUint32(bytes[:], value)
	return netip.AddrFrom4(bytes)
}

func isDefaultPrefix(prefix string) bool {
	return prefix == "0.0.0.0/0" || prefix == "::/0"
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
