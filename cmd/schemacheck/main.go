// Command schemacheck diffs database/schema/schema.sql against whatever
// database DB_DSN actually points at, and reports every missing table,
// missing column, and NOT NULL mismatch in one pass - instead of discovering
// them one at a time as 500 errors in production logs.
//
// Usage:
//
//	go run ./cmd/schemacheck
//	go run ./cmd/schemacheck path/to/other-schema.sql
//
// It reads DB_DSN the same way the server does (.env via godotenv, falling
// back to the DB_DSN environment variable).
//
// This is a diagnostic aid, not a migration tool: it doesn't touch your
// database, and it doesn't check everything (indexes, CHECK constraint
// bodies, and foreign keys aren't parsed) - just table/column presence and
// nullability, which is what's actually been causing the "column X does not
// exist" 500s. The schema.sql parser is a deliberately simple line-based
// heuristic (see parseSchemaFile) that assumes the file is formatted the way
// every CREATE TABLE block in this repo already is: one column or
// constraint per line, table closed by a line that is just ");".
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

type expectedColumn struct {
	name     string
	nullable bool
}

// columnKeyword lines are table-level constraints, not columns - skip them.
var columnKeywords = map[string]bool{
	"constraint": true,
	"check":      true,
	"unique":     true,
	"primary":    true,
	"foreign":    true,
	"exclude":    true,
}

var identifierRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
var createTableRe = regexp.MustCompile(`(?i)^CREATE TABLE IF NOT EXISTS\s+(\w+)\s*\($`)

func main() {
	_ = godotenv.Load()

	schemaPath := "database/schema/schema.sql"
	if len(os.Args) > 1 {
		schemaPath = os.Args[1]
	}

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN is not set (checked .env and the environment)")
	}

	expected, err := parseSchemaFile(schemaPath)
	if err != nil {
		log.Fatalf("failed to parse %s: %v", schemaPath, err)
	}
	if len(expected) == 0 {
		log.Fatalf("parsed zero tables out of %s - check the path is right", schemaPath)
	}
	fmt.Printf("Parsed %d expected table(s) from %s\n\n", len(expected), schemaPath)

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	actualTables, err := fetchActualTables(ctx, conn)
	if err != nil {
		log.Fatalf("failed to list tables: %v", err)
	}
	actualColumns, err := fetchActualColumns(ctx, conn)
	if err != nil {
		log.Fatalf("failed to list columns: %v", err)
	}

	problems := 0
	problems += reportMissingAndExtraTables(expected, actualTables)
	problems += reportColumnDiffs(expected, actualTables, actualColumns)

	fmt.Println()
	if problems == 0 {
		fmt.Println("No mismatches found - the database matches schema.sql for everything this tool checks.")
		return
	}
	fmt.Printf("%d issue(s) found. Fix these before they show up as 500s.\n", problems)
	os.Exit(1)
}

// parseSchemaFile extracts table name -> expected columns from a schema.sql
// formatted the way this repo's is: one CREATE TABLE IF NOT EXISTS block per
// table, one column/constraint per line, closed by a lone ");" line.
func parseSchemaFile(path string) (map[string][]expectedColumn, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := map[string][]expectedColumn{}
	var currentTable string
	inTable := false

	for _, rawLine := range strings.Split(string(data), "\n") {
		line := rawLine
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		trimmed := strings.TrimSpace(line)

		if !inTable {
			if m := createTableRe.FindStringSubmatch(trimmed); m != nil {
				currentTable = strings.ToLower(m[1])
				inTable = true
				result[currentTable] = nil
			}
			continue
		}

		if trimmed == ");" {
			inTable = false
			continue
		}
		if trimmed == "" {
			continue
		}

		fields := strings.Fields(trimmed)
		firstLower := strings.ToLower(strings.TrimSuffix(fields[0], ","))
		if columnKeywords[firstLower] {
			continue // table-level constraint line, not a column
		}
		if !identifierRe.MatchString(firstLower) {
			continue // continuation of a multi-line constraint (e.g. a CHECK body)
		}

		upper := strings.ToUpper(trimmed)
		nullable := !strings.Contains(upper, "NOT NULL") && !strings.Contains(upper, "PRIMARY KEY")

		result[currentTable] = append(result[currentTable], expectedColumn{
			name:     firstLower,
			nullable: nullable,
		})
	}

	return result, nil
}

func fetchActualTables(ctx context.Context, conn *pgx.Conn) (map[string]bool, error) {
	rows, err := conn.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables[name] = true
	}
	return tables, rows.Err()
}

type actualColumn struct {
	nullable bool
}

func fetchActualColumns(ctx context.Context, conn *pgx.Conn) (map[string]map[string]actualColumn, error) {
	rows, err := conn.Query(ctx, `
		SELECT table_name, column_name, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := map[string]map[string]actualColumn{}
	for rows.Next() {
		var table, column, isNullable string
		if err := rows.Scan(&table, &column, &isNullable); err != nil {
			return nil, err
		}
		if cols[table] == nil {
			cols[table] = map[string]actualColumn{}
		}
		cols[table][column] = actualColumn{nullable: isNullable == "YES"}
	}
	return cols, rows.Err()
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func reportMissingAndExtraTables(expected map[string][]expectedColumn, actualTables map[string]bool) int {
	problems := 0
	for _, table := range sortedKeys(expected) {
		if !actualTables[table] {
			fmt.Printf("MISSING TABLE: %s (in schema.sql, not in the database)\n", table)
			problems++
		}
	}
	for table := range actualTables {
		if _, ok := expected[table]; !ok {
			fmt.Printf("EXTRA TABLE (informational, not necessarily wrong): %s (in the database, not in schema.sql)\n", table)
		}
	}
	return problems
}

func reportColumnDiffs(expected map[string][]expectedColumn, actualTables map[string]bool, actualColumns map[string]map[string]actualColumn) int {
	problems := 0
	for _, table := range sortedKeys(expected) {
		if !actualTables[table] {
			continue // already reported as a missing table
		}
		actualCols := actualColumns[table]

		seen := map[string]bool{}
		for _, col := range expected[table] {
			seen[col.name] = true
			actual, ok := actualCols[col.name]
			if !ok {
				fmt.Printf("MISSING COLUMN: %s.%s (in schema.sql, not in the database)\n", table, col.name)
				problems++
				continue
			}
			if actual.nullable != col.nullable {
				fmt.Printf("NULLABLE MISMATCH: %s.%s - schema.sql says %s, database says %s\n",
					table, col.name, nullabilityLabel(col.nullable), nullabilityLabel(actual.nullable))
				problems++
			}
		}

		for colName := range actualCols {
			if !seen[colName] {
				fmt.Printf("EXTRA COLUMN (informational, not necessarily wrong): %s.%s (in the database, not in schema.sql)\n", table, colName)
			}
		}
	}
	return problems
}

func nullabilityLabel(nullable bool) string {
	if nullable {
		return "NULLABLE"
	}
	return "NOT NULL"
}
