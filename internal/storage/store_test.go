package storage

import (
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
