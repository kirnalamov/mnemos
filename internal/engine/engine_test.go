package engine

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/kirnalamov/mnemos/internal/storage"
)

func TestEnginePutGetDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	e, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	if err := e.Put("note-1", []byte("hello")); err != nil {
		t.Fatal(err)
	}

	got, err := e.Get("note-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("Get() = %q, want %q", got, "hello")
	}

	if err := e.Delete("note-1"); err != nil {
		t.Fatal(err)
	}

	_, err = e.Get("note-1")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Get() error = %v, want %v", err, storage.ErrNotFound)
	}
}

func TestEnginePersistsDataAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	e, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := e.Put("note-1", []byte("persistent")); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}

	e, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	got, err := e.Get("note-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "persistent" {
		t.Fatalf("Get() = %q, want %q", got, "persistent")
	}
}
