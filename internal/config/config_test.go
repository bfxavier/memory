package config

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	settings := Default()
	settings.Extraction.Provider = "openai"
	settings.Extraction.Model = "local-model"
	settings.Extraction.APIKeyEnv = "MEMORY_TEST_KEY"
	if err := Save(path, settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Extraction.Enabled || loaded.Extraction.Model != "local-model" || loaded.Extraction.MaxMemories != 8 {
		t.Fatalf("unexpected config: %#v", loaded)
	}
}

func TestEnabledProviderRequiresModel(t *testing.T) {
	settings := Default()
	settings.Extraction.Provider = "openai"
	settings.Extraction.Model = ""
	if err := settings.Validate(); err == nil {
		t.Fatal("expected missing model error")
	}
}
