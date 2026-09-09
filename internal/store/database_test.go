package store

import (
	"net/url"
	"testing"
)

func TestSQLiteFilePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "unix", path: "/tmp/memory.db", want: "/tmp/memory.db"},
		{name: "windows drive", path: "C:/Users/Bruno Xavier/memory.db", want: "/C:/Users/Bruno Xavier/memory.db"},
		{name: "windows drive already rooted", path: "/C:/Users/Bruno Xavier/memory.db", want: "/C:/Users/Bruno Xavier/memory.db"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sqliteFilePath(test.path); got != test.want {
				t.Fatalf("sqliteFilePath(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestWindowsSQLiteFileURL(t *testing.T) {
	location := url.URL{Scheme: "file", Path: sqliteFilePath("C:/Users/Bruno Xavier/memory.db")}
	if got, want := location.String(), "file:///C:/Users/Bruno%20Xavier/memory.db"; got != want {
		t.Fatalf("SQLite file URL = %q, want %q", got, want)
	}
}
