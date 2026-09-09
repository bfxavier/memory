package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	database, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	store := &Store{db: database}
	if err := store.migrate(); err != nil {
		database.Close()
		return nil, err
	}
	return store, nil
}

func OpenReadOnly(path string) (*Store, error) {
	database, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, err
	}
	return &Store{db: database}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}

func dsn(path string, readOnly bool) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	location := url.URL{Scheme: "file", Path: sqliteFilePath(path)}
	query := location.Query()
	query.Add("_pragma", "busy_timeout(50)")
	query.Add("_pragma", "foreign_keys(1)")
	if readOnly {
		query.Set("mode", "ro")
	} else {
		query.Add("_pragma", "journal_mode(WAL)")
		query.Add("_pragma", "synchronous(NORMAL)")
	}
	location.RawQuery = query.Encode()
	return location.String()
}

func sqliteFilePath(path string) string {
	path = filepath.ToSlash(path)
	if len(path) >= 2 && path[1] == ':' {
		path = "/" + path
	}
	return path
}
