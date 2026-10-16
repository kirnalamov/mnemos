package storage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	want := Record{
		Key:  "documents/report.txt",
		Data: []byte("Hello from Mnemos!"),
	}

	encoded, err := Encode(want)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if got.Key != want.Key {
		t.Fatalf("key: got %q, want %q", got.Key, want.Key)
	}

	if !bytes.Equal(got.Data, want.Data) {
		t.Fatal("decoded data differs from original")
	}
}

func TestDecodeDetectsCorruption(t *testing.T) {
	encoded, err := Encode(Record{
		Key:  "test",
		Data: []byte("original data"),
	})
	if err != nil {
		t.Fatal(err)
	}

	encoded[len(encoded)-1] ^= 0xFF

	_, err = Decode(encoded)
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("got error %v, want ErrChecksum", err)
	}
}

func TestDecodeRejectsTruncatedRecord(t *testing.T) {
	_, err := Decode([]byte{1, 2, 3})
	if !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("got error %v, want ErrInvalidRecord", err)
	}
}
func TestDecodeRejectsOversizedData(t *testing.T) {
	encoded, err := Encode(Record{
		Key:  "test",
		Data: []byte("small data"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// dataSize находится в байтах 10–17 заголовка.
	binary.BigEndian.PutUint64(encoded[10:18], maxDataSize+1)

	_, err = Decode(encoded)
	if !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("got error %v, want ErrInvalidRecord", err)
	}
}

func TestStoreOperationsAfterClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	if err := store.Put("notes/test.txt", []byte("data")); !errors.Is(err, ErrStoreClosed) {
		t.Errorf("Put error: got %v, want %v", err, ErrStoreClosed)
	}

	if _, err := store.Get("notes/test.txt"); !errors.Is(err, ErrStoreClosed) {
		t.Errorf("Get error: got %v, want %v", err, ErrStoreClosed)
	}

}

func TestOpenRejectsCorruptedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Put("notes/test.txt", []byte("valid data")); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Повреждаем первый байт сигнатуры записи.
	if _, err := file.WriteAt([]byte{0x00}, 0); err != nil {
		file.Close()
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err == nil {
		store.Close()
		t.Fatal("expected Open to reject corrupted record")
	}
}
