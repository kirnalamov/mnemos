package storage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
)

const (
	magic       uint32 = 0x4D4E4D53 // "MNMS"
	version     uint16 = 1
	headerSize         = 22
	maxKeySize         = 1 << 20  // 1 MiB
	maxDataSize uint64 = 64 << 20 // 64 MiB
)

var (
	ErrInvalidRecord = errors.New("invalid storage record")
	ErrChecksum      = errors.New("checksum mismatch")
)

type Record struct {
	Key  string
	Data []byte
}

func Encode(r Record) ([]byte, error) {
	if len(r.Key) == 0 || len(r.Key) > maxKeySize {
		return nil, ErrInvalidRecord
	}
	if uint64(len(r.Data)) > maxDataSize {
		return nil, ErrInvalidRecord
	}
	var buf bytes.Buffer

	if err := binary.Write(&buf, binary.BigEndian, magic); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, version); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(r.Key))); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, uint64(len(r.Data))); err != nil {
		return nil, err
	}

	checksum := crc32.ChecksumIEEE(r.Data)
	if err := binary.Write(&buf, binary.BigEndian, checksum); err != nil {
		return nil, err
	}

	buf.WriteString(r.Key)
	buf.Write(r.Data)

	return buf.Bytes(), nil
}

func Decode(src []byte) (Record, error) {
	var r Record

	if len(src) < headerSize {
		return r, ErrInvalidRecord
	}

	reader := bytes.NewReader(src)

	var gotMagic uint32
	var gotVersion uint16
	var keySize uint32
	var dataSize uint64
	var checksum uint32

	if err := binary.Read(reader, binary.BigEndian, &gotMagic); err != nil {
		return r, err
	}
	if err := binary.Read(reader, binary.BigEndian, &gotVersion); err != nil {
		return r, err
	}
	if err := binary.Read(reader, binary.BigEndian, &keySize); err != nil {
		return r, err
	}
	if err := binary.Read(reader, binary.BigEndian, &dataSize); err != nil {
		return r, err
	}
	if err := binary.Read(reader, binary.BigEndian, &checksum); err != nil {
		return r, err
	}

	if gotMagic != magic || gotVersion != version {
		return r, ErrInvalidRecord
	}
	if keySize == 0 || keySize > maxKeySize {
		return r, ErrInvalidRecord
	}

	if dataSize > maxDataSize {
		return r, ErrInvalidRecord
	}

	remaining := uint64(reader.Len())

	if dataSize > remaining {
		return r, ErrInvalidRecord
	}

	if uint64(keySize) != remaining-dataSize {
		return r, ErrInvalidRecord
	}

	key := make([]byte, int(keySize))
	if _, err := io.ReadFull(reader, key); err != nil {
		return r, err
	}

	data := make([]byte, int(dataSize))
	if _, err := io.ReadFull(reader, data); err != nil {
		return r, err
	}

	if crc32.ChecksumIEEE(data) != checksum {
		return r, ErrChecksum
	}

	r.Key = string(key)
	r.Data = data
	return r, nil
}
