package engine

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/kirnalamov/mnemos/internal/storage"
)

type fakeStore struct {
	putFn     func(string, []byte) error
	getFn     func(string) ([]byte, error)
	deleteFn  func(string) error
	closeFn   func() error
	compactFn func() error
}

func (f *fakeStore) Put(key string, data []byte) error {
	return f.putFn(key, data)
}

func (f *fakeStore) Get(key string) ([]byte, error) {
	return f.getFn(key)
}

func (f *fakeStore) Delete(key string) error {
	return f.deleteFn(key)
}

func (f *fakeStore) Close() error {
	return f.closeFn()
}

func (f *fakeStore) Compact() error {
	return f.compactFn()
}

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

func TestEngineDelegatesErrors(t *testing.T) {
	wantErr := errors.New("storage failure")

	fake := &fakeStore{
		putFn: func(string, []byte) error {
			return wantErr
		},
		getFn: func(string) ([]byte, error) {
			return nil, wantErr
		},
		deleteFn: func(string) error {
			return wantErr
		},
		closeFn: func() error {
			return wantErr
		},
		compactFn: func() error {
			return wantErr
		},
	}

	e := &Engine{store: fake}

	if err := e.Put("key", []byte("value")); !errors.Is(err, wantErr) {
		t.Fatalf("Put() error = %v, want %v", err, wantErr)
	}

	if _, err := e.Get("key"); !errors.Is(err, wantErr) {
		t.Fatalf("Get() error = %v, want %v", err, wantErr)
	}

	if err := e.Delete("key"); !errors.Is(err, wantErr) {
		t.Fatalf("Delete() error = %v, want %v", err, wantErr)
	}

	if err := e.Close(); !errors.Is(err, wantErr) {
		t.Fatalf("Close() error = %v, want %v", err, wantErr)
	}

	if err := e.Compact(); !errors.Is(err, wantErr) {
		t.Fatalf("Compact() error = %v, want %v", err, wantErr)
	}
}
