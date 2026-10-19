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

	if entry.Size != len(data)+headerSizeV3+len(key) {
		t.Errorf("size: got %d, want %d",
			entry.Size, len(data)+headerSizeV3+len(key))
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

type failingWriter struct {
	data        []byte
	writeLimit  int
	writeErr    error
	truncateErr error
}

func (w *failingWriter) Write(p []byte) (int, error) {
	n := w.writeLimit
	if n > len(p) {
		n = len(p)
	}

	w.data = append(w.data, p[:n]...)
	return n, w.writeErr
}

func (w *failingWriter) Truncate(size int64) error {
	if w.truncateErr != nil {
		return w.truncateErr
	}
	if size < 0 || size > int64(len(w.data)) {
		return errors.New("invalid truncate size")
	}

	w.data = w.data[:size]
	return nil
}

type failingFile struct {
	*os.File

	writeLimit  int
	writeErr    error
	truncateErr error
}

func (f *failingFile) Write(p []byte) (int, error) {
	n := f.writeLimit
	if n > len(p) {
		n = len(p)
	}

	written, err := f.File.Write(p[:n])
	if err != nil {
		return written, err
	}

	return written, f.writeErr
}

func (f *failingFile) Truncate(size int64) error {
	if f.truncateErr != nil {
		return f.truncateErr
	}

	return f.File.Truncate(size)
}

type syncFailingFile struct {
	*failingFile
	syncErr error
}

func (f *syncFailingFile) Sync() error {
	return f.syncErr
}

func TestWriteRecordRollsBackPartialWrite(t *testing.T) {
	initial := []byte("existing data")
	w := &failingWriter{
		data:       append([]byte(nil), initial...),
		writeLimit: 3,
		writeErr:   errors.New("simulated write failure"),
	}

	err := writeRecord(w, int64(len(initial)), []byte("new record"))
	if err == nil {
		t.Fatal("expected write error")
	}

	if got, want := string(w.data), string(initial); got != want {
		t.Fatalf("data after rollback: got %q, want %q", got, want)
	}
}

func TestStoreEntersFailedStateAfterRollbackFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_RDWR|os.O_APPEND,
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}

	writeErr := errors.New("simulated write failure")
	truncateErr := errors.New("simulated truncate failure")

	failedFile := &failingFile{
		File:        file,
		writeLimit:  3,
		writeErr:    writeErr,
		truncateErr: truncateErr,
	}

	store := &Store{
		file:  failedFile,
		index: make(map[string]Entry),
	}

	err = store.Put("notes/test.txt", []byte("hello"))
	if !errors.Is(err, ErrRollbackFailed) {
		t.Fatalf("Put error: got %v, want ErrRollbackFailed", err)
	}

	if !store.failed {
		t.Fatal("store should enter failed state")
	}

	if err := store.Put("notes/another.txt", []byte("data")); !errors.Is(err, ErrStoreFailed) {
		t.Errorf("second Put error: got %v, want ErrStoreFailed", err)
	}

	if _, err := store.Get("notes/test.txt"); !errors.Is(err, ErrStoreFailed) {
		t.Errorf("Get error: got %v, want ErrStoreFailed", err)
	}

	if err := store.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}

	if _, err := store.Get("notes/test.txt"); !errors.Is(err, ErrStoreClosed) {
		t.Errorf("Get after Close: got %v, want ErrStoreClosed", err)
	}
}

func TestStoreDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Put("note-1", []byte("hello")); err != nil {
		t.Fatal(err)
	}

	if err := store.Delete("note-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.Get("note-1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: got %v, want %v", err, ErrNotFound)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store.Close()

	_, err = store.Get("note-1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after reopen: got %v, want %v", err, ErrNotFound)
	}
}

func TestStoreDeleteNotFound(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "data.mnemos"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	err = store.Delete("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete missing key: got %v, want %v", err, ErrNotFound)
	}
}

func TestStoreDeleteDoesNotRemoveNewerValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Put("note-1", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("note-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("note-1", []byte("second")); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get("note-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second" {
		t.Fatalf("Get: got %q, want %q", got, "second")
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	got, err = store.Get("note-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second" {
		t.Fatalf("Get after reopen: got %q, want %q", got, "second")
	}
}

func TestStoreOpenRejectsIncompleteTailHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Put("note-1", []byte("saved data")); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	validSize := info.Size()

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte{1, 2, 3}); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err == nil {
		store.Close()
		t.Fatal("expected Open to reject incomplete tail header")
	}

	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != validSize+3 {
		t.Fatalf("file size: got %d, want %d", info.Size(), validSize+3)
	}
}

func TestStoreOpenRejectsIncompleteTailRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Put("note-1", []byte("saved data")); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	validSize := info.Size()

	partial, err := Encode(Record{
		Key:  "note-2",
		Data: []byte("incomplete data"),
	})
	if err != nil {
		t.Fatal(err)
	}
	partial = partial[:len(partial)-1]

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(partial); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err == nil {
		store.Close()
		t.Fatal("expected Open to reject incomplete record")
	}

	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	wantSize := validSize + int64(len(partial))
	if info.Size() != wantSize {
		t.Fatalf("file size: got %d, want %d", info.Size(), wantSize)
	}
}
func TestStoreEntersFailedStateAfterSyncFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Put("notes/test.txt", []byte("original")); err != nil {
		t.Fatal(err)
	}

	oldEntry := store.index["notes/test.txt"]

	syncErr := errors.New("simulated sync failure")
	file, ok := store.file.(*os.File)
	if !ok {
		t.Fatalf("unexpected storage file type: %T", store.file)
	}

	store.file = &syncFailingFile{
		failingFile: &failingFile{
			File:       file,
			writeLimit: int(^uint(0) >> 1),
		},
		syncErr: syncErr,
	}
	err = store.Put("notes/test.txt", []byte("updated"))
	if err == nil {
		t.Fatal("expected Sync error")
	}

	if !errors.Is(err, syncErr) {
		t.Fatalf("Put error: got %v, want Sync error %v", err, syncErr)
	}

	if !store.failed {
		t.Fatal("store should enter failed state after Sync failure")
	}

	if got := store.index["notes/test.txt"]; got != oldEntry {
		t.Fatal("index entry changed after Sync failure")
	}

	if err := store.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
}

func TestStoreDeleteKeepsIndexAfterSyncFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	key := "notes/test.txt"
	if err := store.Put(key, []byte("original")); err != nil {
		t.Fatal(err)
	}

	oldEntry, ok := store.index[key]
	if !ok {
		t.Fatal("expected object to exist in index")
	}

	file, ok := store.file.(*os.File)
	if !ok {
		t.Fatalf("unexpected storage file type: %T", store.file)
	}

	syncErr := errors.New("simulated sync failure")
	store.file = &syncFailingFile{
		failingFile: &failingFile{
			File:       file,
			writeLimit: int(^uint(0) >> 1),
		},
		syncErr: syncErr,
	}

	err = store.Delete(key)
	if err == nil {
		t.Fatal("expected Sync error")
	}

	if !errors.Is(err, syncErr) {
		t.Fatalf("Delete error: got %v, want Sync error %v", err, syncErr)
	}

	if !store.failed {
		t.Fatal("store should enter failed state after Sync failure")
	}

	gotEntry, ok := store.index[key]
	if !ok {
		t.Fatal("index entry was removed after Sync failure")
	}

	if gotEntry != oldEntry {
		t.Fatalf("index entry changed: got %+v, want %+v", gotEntry, oldEntry)
	}

	if err := store.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
}

func TestStoreReopenAfterPutSyncFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	key := "notes/test.txt"
	if err := store.Put(key, []byte("original")); err != nil {
		t.Fatal(err)
	}

	file, ok := store.file.(*os.File)
	if !ok {
		t.Fatalf("unexpected storage file type: %T", store.file)
	}

	syncErr := errors.New("simulated sync failure")
	store.file = &syncFailingFile{
		failingFile: &failingFile{
			File:       file,
			writeLimit: int(^uint(0) >> 1),
		},
		syncErr: syncErr,
	}

	err = store.Put(key, []byte("updated"))
	if !errors.Is(err, syncErr) {
		t.Fatalf("Put error: got %v, want Sync error %v", err, syncErr)
	}

	if !store.failed {
		t.Fatal("store should enter failed state after Sync failure")
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	recovered, err := Open(path)
	if err != nil {
		t.Fatalf("reopen storage: %v", err)
	}
	defer recovered.Close()

	got, err := recovered.Get(key)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}

	if string(got) != "updated" {
		t.Fatalf("data after reopen: got %q, want %q", got, "updated")
	}
}

func TestStoreReopenAfterDeleteSyncFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	key := "notes/test.txt"
	if err := store.Put(key, []byte("original")); err != nil {
		t.Fatal(err)
	}

	file, ok := store.file.(*os.File)
	if !ok {
		t.Fatalf("unexpected storage file type: %T", store.file)
	}

	syncErr := errors.New("simulated sync failure")
	store.file = &syncFailingFile{
		failingFile: &failingFile{
			File:       file,
			writeLimit: int(^uint(0) >> 1),
		},
		syncErr: syncErr,
	}

	err = store.Delete(key)
	if !errors.Is(err, syncErr) {
		t.Fatalf("Delete error: got %v, want Sync error %v", err, syncErr)
	}

	if !store.failed {
		t.Fatal("store should enter failed state after Sync failure")
	}

	if _, ok := store.index[key]; !ok {
		t.Fatal("index entry should remain until storage is reopened")
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	recovered, err := Open(path)
	if err != nil {
		t.Fatalf("reopen storage: %v", err)
	}
	defer recovered.Close()

	_, err = recovered.Get(key)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after reopen: got %v, want %v", err, ErrNotFound)
	}
}
