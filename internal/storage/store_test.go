package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStorePutWritesRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	key := "notes/hello.txt"
	data := []byte("Hello from Mnemos!")

	if err := store.Put(key, data); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if got.Key != key {
		t.Fatalf("key: got %q, want %q", got.Key, key)
	}

	if string(got.Data) != string(data) {
		t.Fatalf("data: got %q, want %q", got.Data, data)
	}
}

func TestStorePutUpdatesIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	key := "notes/hello.txt"
	data := []byte("Hello from Mnemos!")

	if err := store.Put(key, data); err != nil {
		t.Fatal(err)
	}

	entry, ok := store.index[key]
	if !ok {
		t.Fatal("key not found in index")
	}

	if entry.Offset != 0 {
		t.Errorf("offset: got %d, want 0", entry.Offset)
	}

	if entry.Size != len(data)+headerSize+len(key) {
		t.Errorf("size: got %d, want %d",
			entry.Size, len(data)+headerSize+len(key))
	}
}

func TestStoreGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	key := "notes/hello.txt"
	data := []byte("Hello from Mnemos!")

	if err := store.Put(key, data); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(key)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(data) {
		t.Errorf("data: got %q, want %q", got, data)
	}
}

func TestStoreGetNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_, err = store.Get("missing.txt")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error: got %v, want %v", err, ErrNotFound)
	}
}

func TestStoreGetAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	key := "notes/reopen.txt"
	data := []byte("Persisted data")

	if err := store.Put(key, data); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	got, err := store.Get(key)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(data) {
		t.Fatalf("data: got %q, want %q", got, data)
	}
}

func TestStorePutOverwritesExistingKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	key := "notes/example.txt"

	if err := store.Put(key, []byte("first version")); err != nil {
		t.Fatal(err)
	}

	if err := store.Put(key, []byte("second version")); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(key)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != "second version" {
		t.Fatalf("data: got %q, want %q", got, "second version")
	}
}

func TestStoreReopenUsesLatestVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	key := "notes/example.txt"

	if err := store.Put(key, []byte("first version")); err != nil {
		t.Fatal(err)
	}

	if err := store.Put(key, []byte("second version")); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	got, err := store.Get(key)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != "second version" {
		t.Fatalf("data: got %q, want %q", got, "second version")
	}
}
