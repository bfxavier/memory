package spool

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bfxavier/memory/internal/model"
)

const MaxEventBytes = 64 * 1024

type Consumer func(model.Event) error

func Append(dir string, event model.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if len(data) > MaxEventBytes {
		return fmt.Errorf("event exceeds %d bytes", MaxEventBytes)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	base := event.CreatedAt.UTC().Format("20060102T150405.000000000Z") + "-" + event.ID
	temporary := filepath.Join(dir, "."+base+".tmp")
	final := filepath.Join(dir, base+".json")
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, final); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func Consume(incoming, failed string, consume Consumer) (int, error) {
	entries, err := os.ReadDir(incoming)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	processed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(incoming, entry.Name())
		data, readErr := os.ReadFile(path)
		var event model.Event
		decodeErr := json.Unmarshal(data, &event)
		if readErr != nil || decodeErr != nil || event.ID == "" {
			if err := park(path, failed); err != nil {
				return processed, err
			}
			continue
		}
		if err := consume(event); err != nil {
			return processed, err
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func park(path, failed string) error {
	if err := os.MkdirAll(failed, 0o700); err != nil {
		return err
	}
	target := filepath.Join(failed, filepath.Base(path))
	if err := os.Rename(path, target); err == nil {
		return nil
	}
	return os.Remove(path)
}
