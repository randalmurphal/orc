package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/randalmurphal/orc/internal/db/driver"
)

type legacyRuntimeColumn struct {
	table   string
	oldName string
	newName string
}

var legacyRuntimeColumns = []legacyRuntimeColumn{
	{table: "phase_templates", oldName: "claude_config", newName: "runtime_config"},
	{table: "workflow_phases", oldName: "claude_config_override", newName: "runtime_config_override"},
	{table: "agents", oldName: "claude_config", newName: "runtime_config"},
}

func (d *DB) migrateLegacyRuntimeConfigColumns(ctx context.Context, schemaType string) error {
	if schemaType != "global" && schemaType != "project" {
		return nil
	}

	for _, column := range legacyRuntimeColumns {
		if err := d.renameColumnIfNeeded(ctx, column.table, column.oldName, column.newName); err != nil {
			return err
		}
	}

	return nil
}

func (d *DB) renameColumnIfNeeded(ctx context.Context, table, oldName, newName string) error {
	newExists, err := d.tableHasColumn(ctx, table, newName)
	if err != nil {
		return fmt.Errorf("check %s.%s: %w", table, newName, err)
	}
	if newExists {
		return nil
	}

	oldExists, err := d.tableHasColumn(ctx, table, oldName)
	if err != nil {
		return fmt.Errorf("check %s.%s: %w", table, oldName, err)
	}
	if !oldExists {
		return nil
	}

	query := fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", table, oldName, newName)
	if _, err := d.driver.Exec(ctx, query); err != nil {
		return fmt.Errorf("rename %s.%s to %s: %w", table, oldName, newName, err)
	}

	return nil
}

func (d *DB) tableHasColumn(ctx context.Context, table, column string) (bool, error) {
	switch d.Dialect() {
	case driver.DialectPostgres:
		var exists bool
		err := d.driver.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = $1
				  AND column_name = $2
			)
		`, table, column).Scan(&exists)
		if err != nil {
			return false, err
		}
		return exists, nil
	default:
		rows, err := d.driver.Query(ctx, fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdentifier(table)))
		if err != nil {
			return false, err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var (
				cid       int
				name      string
				colType   string
				notNull   int
				dfltValue sql.NullString
				pk        int
			)
			if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
				return false, err
			}
			if name == column {
				return true, nil
			}
		}
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
}

func quoteSQLiteIdentifier(name string) string {
	return "'" + name + "'"
}
