package storage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
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

	if got.Type != RecordPut {
		t.Fatalf("type: got %v, want RecordPut", got.Type)
	}
}

func TestEncodeRejectsInvalidRecords(t *testing.T) {
	tests := []struct {
		name   string
		record Record
	}{
		{
			name:   "empty key",
			record: Record{Type: RecordPut, Key: "", Data: []byte("data")},
		},
		{
			name:   "oversized key",
			record: Record{Type: RecordPut, Key: string(bytes.Repeat([]byte("k"), maxKeySize+1))},
		},
		{
			name:   "oversized data",
			record: Record{Type: RecordPut, Key: "test", Data: make([]byte, int(maxDataSize)+1)},
		},
		{
			name:   "unknown record type",
			record: Record{Type: RecordType(99), Key: "test"},
		},
		{
			name:   "delete with data",
			record: Record{Type: RecordDelete, Key: "test", Data: []byte("data")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encode(tt.record)
			if !errors.Is(err, ErrInvalidRecord) {
				t.Fatalf("Encode error: got %v, want ErrInvalidRecord", err)
			}
		})
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

	// В заголовках v2 и v3 поле dataSize находится в байтах 11–18.
	binary.BigEndian.PutUint64(encoded[11:19], maxDataSize+1)

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

func TestOpenRejectsIncompleteTail(t *testing.T) {
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

	// Запоминаем размер файла с незавершённой записью.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	originalSize := info.Size()

	store, err = Open(path)
	if err == nil {
		store.Close()
		t.Fatal("expected Open to reject incomplete tail")
	}

	// Open не должен удалять потенциально повреждённые данные.
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Size() != originalSize {
		t.Fatalf(
			"file size changed: got %d, want %d",
			info.Size(),
			originalSize,
		)
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
func TestOpenRejectsCorruptedTailRecordLengthWithoutTruncating(t *testing.T) {
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

	// В v3 поле dataSize занимает байты 11–18 заголовка.
	// Увеличиваем длину последней записи на один байт.
	lastOffset := int64(
		headerSizeV3 + len("notes/first.txt") + len("first"),
	)

	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}

	var b [1]byte
	if _, err := file.ReadAt(b[:], lastOffset+18); err != nil {
		file.Close()
		t.Fatal(err)
	}
	b[0]++

	if _, err := file.WriteAt(b[:], lastOffset+18); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err == nil {
		store.Close()
		t.Fatal("expected Open to reject corrupted tail record length")
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() != originalSize {
		t.Fatalf(
			"file size changed: got %d, want %d",
			after.Size(),
			originalSize,
		)
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

	// В v1 контрольная сумма данных находится в байтах 18–21.
	binary.BigEndian.PutUint32(encoded[18:22], crc32.ChecksumIEEE(data))

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

	// Для v1 CRC32 рассчитывается только по данным.
	binary.BigEndian.PutUint32(encoded[18:22], crc32.ChecksumIEEE(data))

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

func TestDecodeV3DetectsKeyCorruption(t *testing.T) {
	encoded, err := Encode(Record{
		Key:  "note-1",
		Data: []byte("important data"),
	})
	if err != nil {
		t.Fatal(err)
	}

	encoded[headerSizeV3] ^= 0x01

	_, err = Decode(encoded)
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("got error %v, want ErrChecksum", err)
	}
}

func TestDecodeV3DetectsRecordTypeCorruption(t *testing.T) {
	encoded, err := Encode(Record{
		Type: RecordPut,
		Key:  "empty-note",
		Data: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	encoded[6] = byte(RecordDelete)

	_, err = Decode(encoded)
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("got error %v, want ErrChecksum", err)
	}
}

func TestStoreReopenV3Record(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	key := "notes/v3.txt"
	data := []byte("data stored in v3")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Put(key, data); err != nil {
		store.Close()
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatalf("Open v3 file after restart: %v", err)
	}
	defer store.Close()

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get v3 record after restart: %v", err)
	}

	if !bytes.Equal(got, data) {
		t.Fatalf("data: got %q, want %q", got, data)
	}
}

func TestOpenRejectsChecksumCorruptionWithoutModifyingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Put("notes/test.txt", []byte("important data")); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Меняем последний байт данных, не пересчитывая CRC.
	corrupted := append([]byte(nil), before...)
	corrupted[len(corrupted)-1] ^= 0x01

	if err := os.WriteFile(path, corrupted, 0644); err != nil {
		t.Fatal(err)
	}

	beforeOpen, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Open(path)
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("Open error: got %v, want ErrChecksum", err)
	}

	afterOpen, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(afterOpen, beforeOpen) {
		t.Fatal("file contents changed after Open rejected checksum corruption")
	}
}

func TestOpenRejectsCorruptionInMiddleWithoutModifyingFile(t *testing.T) {
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

	if err := store.Put("notes/third.txt", []byte("third")); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Размер первой записи известен из формата v3.
	secondOffset := headerSizeV3 + len("notes/first.txt") + len("first")

	// Меняем байт данных второй записи, оставляя её CRC прежней.
	corrupted := append([]byte(nil), before...)
	secondDataOffset := secondOffset + headerSizeV3 + len("notes/second.txt")
	corrupted[secondDataOffset] ^= 0x01

	if err := os.WriteFile(path, corrupted, 0644); err != nil {
		t.Fatal(err)
	}

	beforeOpen, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Open(path)
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("Open error: got %v, want ErrChecksum", err)
	}

	afterOpen, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(afterOpen, beforeOpen) {
		t.Fatal("file contents changed after Open rejected middle-record corruption")
	}
}

func TestDecodeRejectsUnsupportedVersion(t *testing.T) {
	encoded, err := Encode(Record{
		Key:  "notes/test.txt",
		Data: []byte("data"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Поле версии занимает байты 4–5.
	binary.BigEndian.PutUint16(encoded[4:6], 999)

	_, err = Decode(encoded)
	if !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("Decode error: got %v, want ErrInvalidRecord", err)
	}
}

func TestOpenRejectsUnknownRecordTypeWithoutModifyingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Put("notes/test.txt", []byte("data")); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	corrupted := append([]byte(nil), before...)
	corrupted[6] = 99 // Неизвестный тип записи в формате v3.

	if err := os.WriteFile(path, corrupted, 0644); err != nil {
		t.Fatal(err)
	}

	beforeOpen, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Open(path)
	if !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("Open error: got %v, want ErrInvalidRecord", err)
	}

	afterOpen, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(afterOpen, beforeOpen) {
		t.Fatal("file contents changed after Open rejected unknown record type")
	}
}

func TestOpenFailureLeavesFileUnchangedAndAccessible(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.mnemos")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Put("notes/first.txt", []byte("first")); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	corrupted := append([]byte(nil), original...)
	corrupted[0] ^= 0xFF

	if err := os.WriteFile(path, corrupted, 0644); err != nil {
		t.Fatal(err)
	}

	beforeOpen, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	failedStore, err := Open(path)
	if err == nil {
		if failedStore != nil {
			_ = failedStore.Close()
		}
		t.Fatal("Open succeeded for a corrupted file")
	}

	if failedStore != nil {
		t.Fatal("Open returned a non-nil Store after failure")
	}

	afterOpen, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file is not accessible after Open failure: %v", err)
	}

	if !bytes.Equal(afterOpen, beforeOpen) {
		t.Fatal("file contents changed after Open failure")
	}
}
