package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreCompactReclaimsHistoricalRecordsAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	if err := store.Put(
		"keep",
		[]byte("an old value that should be reclaimed"),
	); err != nil {
		t.Fatal(err)
	}

	if err := store.Put("keep", []byte("current value")); err != nil {
		t.Fatal(err)
	}

	if err := store.Put(
		"remove",
		[]byte("a deleted value that should be reclaimed"),
	); err != nil {
		t.Fatal(err)
	}

	if err := store.Delete("remove"); err != nil {
		t.Fatal(err)
	}

	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Compact(); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if after.Size() >= before.Size() {
		t.Fatalf(
			"compaction did not reclaim space: before=%d after=%d",
			before.Size(),
			after.Size(),
		)
	}

	got, err := store.Get("keep")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("current value")) {
		t.Fatalf("Get(keep) = %q, want %q", got, "current value")
	}

	if _, err := store.Get("remove"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(remove) error = %v, want %v", err, ErrNotFound)
	}

	if entry := store.index["keep"]; entry.Offset != 0 {
		t.Fatalf("compacted record offset = %d, want 0", entry.Offset)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = nil

	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}

	got, err = store.Get("keep")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("current value")) {
		t.Fatalf(
			"Get(keep) after reopen = %q, want %q",
			got,
			"current value",
		)
	}

	if _, err := store.Get("remove"); !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"Get(remove) after reopen error = %v, want %v",
			err,
			ErrNotFound,
		)
	}
}

func TestStoreCompactEmptyStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	if err := store.Put("temporary", []byte("value")); err != nil {
		t.Fatal(err)
	}

	if err := store.Delete("temporary"); err != nil {
		t.Fatal(err)
	}

	if err := store.Compact(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Size() != 0 {
		t.Fatalf("compacted empty store size = %d, want 0", info.Size())
	}

	if len(store.index) != 0 {
		t.Fatalf(
			"index size after compacting empty store = %d, want 0",
			len(store.index),
		)
	}
}

func TestStoreCompactRejectsClosedAndFailedStores(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	if err := store.Compact(); !errors.Is(err, ErrStoreClosed) {
		t.Fatalf(
			"Compact() on closed store error = %v, want %v",
			err,
			ErrStoreClosed,
		)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}

	store.failed = true
	if err := store.Compact(); !errors.Is(err, ErrStoreFailed) {
		t.Fatalf(
			"Compact() on failed store error = %v, want %v",
			err,
			ErrStoreFailed,
		)
	}

	store.failed = false
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreCompactDoesNotReplaceCorruptedLiveRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = store.Close()
	}()

	key := "keep"
	if err := store.Put(key, []byte("payload")); err != nil {
		t.Fatal(err)
	}

	mutator, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Change a payload byte without updating the v3 checksum.
	if _, err := mutator.WriteAt(
		[]byte{'X'},
		int64(headerSizeV3+len(key)),
	); err != nil {
		_ = mutator.Close()
		t.Fatal(err)
	}

	if err := mutator.Close(); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Compact(); !errors.Is(err, ErrChecksum) {
		t.Fatalf("Compact() error = %v, want %v", err, ErrChecksum)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(after, before) {
		t.Fatal("compaction modified the original file after detecting corruption")
	}
}
