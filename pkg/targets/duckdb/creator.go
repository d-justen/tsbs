package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/marcboeker/go-duckdb/v2"
	_ "github.com/marcboeker/go-duckdb/v2"
	"github.com/timescale/tsbs/pkg/targets"
)

const (
	tagsKey = "tags"
)

var tableCols = make(map[string][]string)

type dbCreator struct {
	ds targets.DataSource
}

func (d *dbCreator) Init() {
	d.ds.Headers()
}

func (d *dbCreator) DBExists(dbName string) bool {
	connector, err := duckdb.NewConnector("tsbs.duckdb", nil)
	db := sql.OpenDB(connector)
	defer db.Close()
	rows, err := db.QueryContext(context.Background(), "SHOW TABLES")
	if err != nil {
		panic(err)
	}

	defer rows.Close()
	for rows.Next() {
		return true
	}

	return false
}

func (d *dbCreator) RemoveOldDB(dbName string) error {
	connector, err := duckdb.NewConnector("tsbs.duckdb", nil)
	db := sql.OpenDB(connector)
	defer db.Close()
	rows, err := db.QueryContext(context.Background(), "SHOW TABLES")
	if err != nil {
		return err
	}

	defer rows.Close()
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return err
		}
		_, err := db.ExecContext(context.Background(), fmt.Sprintf("DROP TABLE %s", tableName))
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *dbCreator) CreateDB(dbName string) error {
	time.Sleep(time.Second)
	return nil
}

func (d *dbCreator) PostCreateDB(dbName string) error {
	connector, err := duckdb.NewConnector("tsbs.duckdb", nil)
	db := sql.OpenDB(connector)
	defer db.Close()

	headers := d.ds.Headers()

	tagNames := headers.TagKeys
	tagTypes := headers.TagTypes
	// Create tags table
	_, err = db.ExecContext(context.Background(), "DROP TABLE IF EXISTS tags")
	if err != nil {
		return err
	}

	tagColumnDefinitions := make([]string, len(tagNames))
	for i, tagName := range tagNames {
		pgType := serializedTypeToDuckDBType(tagTypes[i])
		tagColumnDefinitions[i] = fmt.Sprintf("%s %s", tagName, pgType)
	}
	cols := strings.Join(tagColumnDefinitions, ", ")
	_, err = db.ExecContext(context.Background(), "CREATE OR REPLACE SEQUENCE id_sequence START 1")
	if err != nil {
		return err
	}

	_, err = db.ExecContext(context.Background(), fmt.Sprintf(
		"CREATE TABLE tags (id INTEGER DEFAULT nextval('id_sequence'), %s)", cols))
	if err != nil {
		return err
	}

	// tableCols is a global map. Globally cache the available tags
	tableCols[tagsKey] = tagNames

	// Each table is defined in the dbCreator 'cols' list. The definition consists of a
	// comma separated list of the table name followed by its columns. Iterate over each
	// definition to update our global cache and create the requisite tables and indexes
	for tableName, columns := range headers.FieldKeys {
		// tableCols is a global map. Globally cache the available columns for the given table
		tableCols[tableName] = columns
		fieldDefs := d.getFieldDefinitions(tableName, columns)

		_, err := db.ExecContext(context.Background(), fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
		if err != nil {
			return err
		}

		_, err = db.ExecContext(context.Background(), fmt.Sprintf(
			"CREATE TABLE %s (time TIMESTAMPTZ, tags_id integer, %s)", tableName, strings.Join(fieldDefs, ",")))
		if err != nil {
			return err
		}
	}

	return nil
}

func serializedTypeToDuckDBType(serializedType string) string {
	switch serializedType {
	case "string":
		return "TEXT"
	case "float32":
		return "FLOAT"
	case "float64":
		return "DOUBLE"
	case "int64":
		return "BIGINT"
	case "int32":
		return "INTEGER"
	default:
		panic(fmt.Sprintf("unrecognized type %s", serializedType))
	}
}

func (d *dbCreator) getFieldDefinitions(tableName string, columns []string) []string {
	var fieldDefs []string
	var allCols []string

	allCols = append(allCols, columns...)
	for _, field := range allCols {
		if len(field) == 0 {
			continue
		}
		fieldType := "DOUBLE"

		fieldDefs = append(fieldDefs, fmt.Sprintf("%s %s", field, fieldType))
	}
	return fieldDefs
}
