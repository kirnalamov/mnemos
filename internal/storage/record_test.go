package storage

import (
	"bytes"
	"encoding/binary"
	"errors"
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
