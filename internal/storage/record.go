package storage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
)

const (
	magic        uint32 = 0x4D4E4D53 // "MNMS"
	versionV1    uint16 = 1
	versionV2    uint16 = 2
	versionV3    uint16 = 3
	headerSizeV1        = 22
	headerSizeV2        = 23
	headerSizeV3        = 23
	maxKeySize          = 1 << 20  // 1 MiB
	maxDataSize  uint64 = 64 << 20 // 64 MiB
	version      uint16 = versionV3
	headerSize          = headerSizeV3
)

type RecordType uint8

const (
	RecordPut RecordType = iota
	RecordDelete
)

var (
	ErrInvalidRecord = errors.New("invalid storage record")
	ErrChecksum      = errors.New("checksum mismatch")
)

type Record struct {
	Type RecordType
	Key  string
	Data []byte
}

func checksumV3(encoded []byte) uint32 {
	hash := crc32.NewIEEE()
	_, _ = hash.Write(encoded[6:19])
	_, _ = hash.Write(encoded[headerSizeV3:])
	return hash.Sum32()
}

func Encode(r Record) ([]byte, error) {
	if len(r.Key) == 0 || len(r.Key) > maxKeySize {
		return nil, ErrInvalidRecord
	}
	if uint64(len(r.Data)) > maxDataSize {
		return nil, ErrInvalidRecord
	}
	if r.Type != RecordPut && r.Type != RecordDelete {
		return nil, ErrInvalidRecord
	}
	if r.Type == RecordDelete && len(r.Data) != 0 {
		return nil, ErrInvalidRecord
	}

	var buf bytes.Buffer

	if err := binary.Write(&buf, binary.BigEndian, magic); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, versionV3); err != nil {
		return nil, err
	}
	if err := buf.WriteByte(byte(r.Type)); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(r.Key))); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, uint64(len(r.Data))); err != nil {
		return nil, err
	}

	// Reserve the checksum field; fill it after appending key and data.
	if err := binary.Write(&buf, binary.BigEndian, uint32(0)); err != nil {
		return nil, err
	}

	buf.WriteString(r.Key)
	buf.Write(r.Data)

	encoded := buf.Bytes()
	binary.BigEndian.PutUint32(encoded[19:23], checksumV3(encoded))

	return encoded, nil
}

func Decode(src []byte) (Record, error) {
	var r Record

	if len(src) < headerSizeV1 {
		return r, ErrInvalidRecord
	}

	reader := bytes.NewReader(src)

	var gotMagic uint32
	var gotVersion uint16

	if err := binary.Read(reader, binary.BigEndian, &gotMagic); err != nil {
		return r, ErrInvalidRecord
	}
	if err := binary.Read(reader, binary.BigEndian, &gotVersion); err != nil {
		return r, ErrInvalidRecord
	}
	if gotMagic != magic {
		return r, ErrInvalidRecord
	}

	switch gotVersion {
	case versionV1:
		r.Type = RecordPut
	case versionV2, versionV3:
		if len(src) < headerSizeV2 {
			return r, ErrInvalidRecord
		}
		recordType, err := reader.ReadByte()
		if err != nil {
			return r, ErrInvalidRecord
		}
		r.Type = RecordType(recordType)
		if r.Type != RecordPut && r.Type != RecordDelete {
			return r, ErrInvalidRecord
		}
	default:
		return r, ErrInvalidRecord
	}

	var keySize uint32
	var dataSize uint64
	var checksum uint32

	if err := binary.Read(reader, binary.BigEndian, &keySize); err != nil {
		return r, ErrInvalidRecord
	}
	if err := binary.Read(reader, binary.BigEndian, &dataSize); err != nil {
		return r, ErrInvalidRecord
	}
	if err := binary.Read(reader, binary.BigEndian, &checksum); err != nil {
		return r, ErrInvalidRecord
	}

	if keySize == 0 || keySize > maxKeySize || dataSize > maxDataSize {
		return r, ErrInvalidRecord
	}
	if r.Type == RecordDelete && dataSize != 0 {
		return r, ErrInvalidRecord
	}

	remaining := uint64(reader.Len())
	if dataSize > remaining || uint64(keySize) != remaining-dataSize {
		return r, ErrInvalidRecord
	}

	key := make([]byte, int(keySize))
	if _, err := io.ReadFull(reader, key); err != nil {
		return r, ErrInvalidRecord
	}

	data := make([]byte, int(dataSize))
	if _, err := io.ReadFull(reader, data); err != nil {
		return r, ErrInvalidRecord
	}

	switch gotVersion {
	case versionV1, versionV2:
		// Legacy formats protect only the data payload.
		if crc32.ChecksumIEEE(data) != checksum {
			return r, ErrChecksum
		}
	case versionV3:
		// V3 protects record type, lengths, key and data.
		if checksumV3(src) != checksum {
			return r, ErrChecksum
		}
	}

	r.Key = string(key)
	r.Data = data
	return r, nil
}
