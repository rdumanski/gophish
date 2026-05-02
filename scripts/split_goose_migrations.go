// One-time tool that splits goose-format SQL migrations into golang-migrate
// pairs (<version>_<name>.up.sql and <version>_<name>.down.sql).
//
// Usage:
//   go run scripts/split_goose_migrations.go db/db_sqlite3/migrations
//   go run scripts/split_goose_migrations.go db/db_mysql/migrations
//
// Goose format (single file):
//   -- +goose Up
//   ... SQL ...
//   -- +goose Down
//   ... SQL ...
//
// golang-migrate format (two files):
//   <name>.up.sql
//   <name>.down.sql
//
// The script:
//   - reads every *.sql file in the given directory whose name does NOT already
//     contain ".up." or ".down." (idempotent),
//   - splits the content on the goose section markers,
//   - writes the up/down content into <basename>.up.sql / <basename>.down.sql,
//   - removes the original goose-format file.
//
// After running, the directory contains only golang-migrate pairs. Re-running
// is a no-op.

//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	upMarker   = regexp.MustCompile(`(?m)^\s*--\s*\+goose\s+Up\s*$`)
	downMarker = regexp.MustCompile(`(?m)^\s*--\s*\+goose\s+Down\s*$`)
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: split_goose_migrations <migrations-dir>")
		os.Exit(2)
	}
	dir := os.Args[1]

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dir: %v\n", err)
		os.Exit(1)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		// Skip already-split files.
		if strings.Contains(e.Name(), ".up.sql") || strings.Contains(e.Name(), ".down.sql") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if err := split(path); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("split: %s\n", e.Name())
	}
}

func split(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	upLoc := upMarker.FindIndex(data)
	if upLoc == nil {
		return fmt.Errorf("no `-- +goose Up` marker found")
	}
	downLoc := downMarker.FindIndex(data)

	var upBody, downBody []byte
	if downLoc == nil {
		// Up only; no Down.
		upBody = data[upLoc[1]:]
		downBody = []byte("-- IRREVERSIBLE: this migration had no Down section in goose.\n")
	} else if downLoc[0] < upLoc[0] {
		return fmt.Errorf("Down marker appears before Up marker (unexpected)")
	} else {
		upBody = data[upLoc[1]:downLoc[0]]
		downBody = data[downLoc[1]:]
	}

	upBody = trimEdges(upBody)
	downBody = trimEdges(downBody)

	base := strings.TrimSuffix(path, ".sql")
	upPath := base + ".up.sql"
	downPath := base + ".down.sql"

	if err := os.WriteFile(upPath, upBody, 0o644); err != nil {
		return fmt.Errorf("write up: %w", err)
	}
	if err := os.WriteFile(downPath, downBody, 0o644); err != nil {
		return fmt.Errorf("write down: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove original: %w", err)
	}
	return nil
}

func trimEdges(b []byte) []byte {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return []byte{}
	}
	return []byte(s + "\n")
}
