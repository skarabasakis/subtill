package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/civil"
	"github.com/jmoiron/sqlx"
)

// dateStrings formats dates as YYYY-MM-DD. It never returns nil, so an empty
// input marshals to a JSON [] rather than null.
func dateStrings(dates []civil.Date) []string {
	strs := make([]string, 0, len(dates))
	for _, d := range dates {
		strs = append(strs, d.String())
	}
	return strs
}

func nullableDate(d *civil.Date) *string {
	if d == nil {
		return nil
	}
	s := d.String()
	return &s
}

func parseNullableDate(s *string) (*civil.Date, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	d, err := civil.ParseDate(*s)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// columnList returns columns as a comma-separated list of quoted identifiers,
// so that column names that are keywords, like "group", need no special care.
func columnList(columns []string) string {
	quoted := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted = append(quoted, `"`+column+`"`)
	}
	return strings.Join(quoted, ", ")
}

// jsonObject returns a json_object expression keyed by column name, with the
// values read from the columns of alias.
func jsonObject(alias string, columns []string) string {
	pairs := make([]string, 0, len(columns))
	for _, column := range columns {
		pairs = append(pairs, fmt.Sprintf(`'%s', %s."%s"`, column, alias, column))
	}
	return "json_object(" + strings.Join(pairs, ", ") + ")"
}

// getOne scans the single row of query into dest, mapping no rows to
// ErrNotFound.
func getOne(q sqlx.Ext, dest any, query string, arg any) error {
	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return err
	}
	err = sqlx.Get(q, dest, q.Rebind(query), args...)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func selectAll(q sqlx.Ext, dest any, query string, arg any) error {
	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return err
	}
	return sqlx.Select(q, dest, q.Rebind(query), args...)
}

func exec(q sqlx.Ext, query string, arg any) error {
	_, err := sqlx.NamedExec(q, query, arg)
	return err
}

func execRowsAffected(q sqlx.Ext, query string, arg any) (int64, error) {
	result, err := sqlx.NamedExec(q, query, arg)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
