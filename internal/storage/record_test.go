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

func TestOpenTruncatesIncompleteTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	key := "notes/safe.txt"
	data := []byte("valid data")

	if err := store.Put(key, data); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	// Имитируем аварийное завершение посередине второй записи.
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}

	partial, err := Encode(Record{
		Key:  "notes/incomplete.txt",
		Data: []byte("incomplete data"),
	})
	if err != nil {
		file.Close()
		t.Fatal(err)
	}

	if _, err := file.Write(partial[:len(partial)/2]); err != nil {
		file.Close()
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatalf("Open should recover incomplete tail: %v", err)
	}
	defer store.Close()

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get existing record: %v", err)
	}

	if string(got) != string(data) {
		t.Fatalf("data: got %q, want %q", got, data)
	}

	_, err = store.Get("notes/incomplete.txt")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("incomplete key error: got %v, want %v", err, ErrNotFound)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	expectedSize := int64(headerSizeV2 + len(key) + len(data))
	if info.Size() != expectedSize {
		t.Fatalf("file size: got %d, want %d", info.Size(), expectedSize)
	}

}

func TestOpenRejectsCorruptedRecordWithoutTruncating(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Put("notes/first.txt", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("notes/second.txt", []byte("second")); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	originalSize := before.Size()

	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Повреждаем сигнатуру первой записи.
	if _, err := file.WriteAt([]byte{0x00}, 0); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = Open(path)
	if err == nil {
		t.Fatal("expected Open to reject corrupted record")
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if after.Size() != originalSize {
		t.Fatalf("file size changed: got %d, want %d", after.Size(), originalSize)
	}
}

func TestDecodeLegacyV1Record(t *testing.T) {
	key := []byte("legacy-note")
	data := []byte("data from v1")

	encoded := make([]byte, headerSizeV1+len(key)+len(data))

	binary.BigEndian.PutUint32(encoded[0:4], magic)
	binary.BigEndian.PutUint16(encoded[4:6], versionV1)
	binary.BigEndian.PutUint32(encoded[6:10], uint32(len(key)))
	binary.BigEndian.PutUint64(encoded[10:18], uint64(len(data)))

	// В формате v1 контрольная сумма занимает байты 18–21.
	// Используем Encode для получения корректной CRC32 данных.
	v2, err := Encode(Record{
		Key:  string(key),
		Data: data,
	})
	if err != nil {
		t.Fatal(err)
	}
	copy(encoded[18:22], v2[19:23])

	copy(encoded[headerSizeV1:], key)
	copy(encoded[headerSizeV1+len(key):], data)

	got, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode v1 record: %v", err)
	}

	if got.Type != RecordPut {
		t.Fatalf("type: got %v, want RecordPut", got.Type)
	}
	if got.Key != string(key) {
		t.Fatalf("key: got %q, want %q", got.Key, key)
	}
	if !bytes.Equal(got.Data, data) {
		t.Fatalf("data: got %q, want %q", got.Data, data)
	}
}

func TestStoreReopenLegacyV1Record(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	key := []byte("legacy-note")
	data := []byte("data from v1")

	encoded := make([]byte, headerSizeV1+len(key)+len(data))
	binary.BigEndian.PutUint32(encoded[0:4], magic)
	binary.BigEndian.PutUint16(encoded[4:6], versionV1)
	binary.BigEndian.PutUint32(encoded[6:10], uint32(len(key)))
	binary.BigEndian.PutUint64(encoded[10:18], uint64(len(data)))

	v2, err := Encode(Record{
		Key:  string(key),
		Data: data,
	})
	if err != nil {
		t.Fatal(err)
	}
	copy(encoded[18:22], v2[19:23])
	copy(encoded[headerSizeV1:], key)
	copy(encoded[headerSizeV1+len(key):], data)

	if err := os.WriteFile(path, encoded, 0644); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open legacy v1 file: %v", err)
	}
	defer store.Close()

	got, err := store.Get(string(key))
	if err != nil {
		t.Fatalf("Get legacy key: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("data: got %q, want %q", got, data)
	}
}
