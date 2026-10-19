package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kirnalamov/mnemos/internal/storage"
)

func TestRunPutGetDelete(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("MNEMOS_DATA_DIR", dataDir)

	if err := run([]string{"put", "note-1", "hello"}); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"get", "note-1"}); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"delete", "note-1"}); err != nil {
		t.Fatal(err)
	}

	err := run([]string{"get", "note-1"})
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("run(get) error = %v, want %v", err, storage.ErrNotFound)
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "put without value", args: []string{"put", "note-1"}},
		{name: "get without key", args: []string{"get"}},
		{name: "delete without key", args: []string{"delete"}},
		{name: "unknown command", args: []string{"unknown"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := run(tt.args); err == nil {
				t.Fatal("run() error = nil, want an error")
			}
		})
	}
}

func TestRunRequiresDataDirectory(t *testing.T) {
	t.Setenv("MNEMOS_DATA_DIR", "")

	// godotenv.Load() не должен восстанавливать переменную
	// из окружения, если в .env нет соответствующего значения.
	err := run([]string{"put", "note-1", "hello"})
	if err == nil {
		t.Fatal("run() error = nil, want missing data directory error")
	}
}

func TestRunCreatesDataDirectory(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "nested", "data")
	t.Setenv("MNEMOS_DATA_DIR", dataDir)

	if err := run([]string{"put", "note-1", "hello"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "data.mnemos")); err != nil {
		t.Fatalf("data file was not created: %v", err)
	}
}

func TestValidateArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid put",
			args:    []string{"put", "key", "value"},
			wantErr: false,
		},
		{
			name:    "valid get",
			args:    []string{"get", "key"},
			wantErr: false,
		},
		{
			name:    "valid delete",
			args:    []string{"delete", "key"},
			wantErr: false,
		},
		{
			name:    "put without value",
			args:    []string{"put", "key"},
			wantErr: true,
		},
		{
			name:    "get without key",
			args:    []string{"get"},
			wantErr: true,
		},
		{
			name:    "delete without key",
			args:    []string{"delete"},
			wantErr: true,
		},
		{
			name:    "unknown command",
			args:    []string{"unknown"},
			wantErr: true,
		},
		{
			name:    "valid compact",
			args:    []string{"compact"},
			wantErr: false,
		},
		{
			name:    "compact with extra args",
			args:    []string{"compact", "extra"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArgs(tt.args)

			if (err != nil) != tt.wantErr {
				t.Fatalf("validateArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunHelpDoesNotRequireDataDirectory(t *testing.T) {
	t.Setenv("MNEMOS_DATA_DIR", "")

	for _, args := range [][]string{
		{"help"},
		{"--help"},
		{"-h"},
	} {
		if err := run(args); err != nil {
			t.Errorf("run(%v) error = %v, want nil", args, err)
		}
	}
}

func TestRunCompactReclaimsHistoricalRecords(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("MNEMOS_DATA_DIR", dataDir)

	commands := [][]string{
		{"put", "note-1", "old value that takes space"},
		{"put", "note-1", "latest"},
		{"put", "deleted", "another value that takes space"},
		{"delete", "deleted"},
	}

	for _, args := range commands {
		if err := run(args); err != nil {
			t.Fatalf("run(%v): %v", args, err)
		}
	}

	path := filepath.Join(dataDir, "data.mnemos")

	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"compact"}); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if after.Size() >= before.Size() {
		t.Fatalf(
			"compact did not reduce file size: before=%d after=%d",
			before.Size(),
			after.Size(),
		)
	}

	// Проверяем, что актуальное значение осталось доступно через CLI.
	if err := run([]string{"get", "note-1"}); err != nil {
		t.Fatalf("get latest value after compact: %v", err)
	}

	// Удалённый ключ не должен возвращаться после уплотнения.
	if err := run([]string{"get", "deleted"}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("get deleted key error = %v, want %v", err, storage.ErrNotFound)
	}
}
