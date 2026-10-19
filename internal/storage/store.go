package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var ErrStoreClosed = errors.New("storage is closed")
var ErrNotFound = errors.New("object not found")
var ErrStoreFailed = errors.New("storage is in failed state")
var ErrRollbackFailed = errors.New("rollback after failed write was unsuccessful")

type Entry struct {
	Offset int64
	Size   int
}

type storageFile interface {
	Write([]byte) (int, error)
	Seek(int64, int) (int64, error)
	Truncate(int64) error
	ReadAt([]byte, int64) (int, error)
	Stat() (os.FileInfo, error)
	Sync() error
	Close() error
}

type Store struct {
	mu     sync.Mutex
	file   storageFile
	path   string
	index  map[string]Entry
	closed bool
	failed bool
}

type recordWriter interface {
	Write([]byte) (int, error)
	Truncate(int64) error
}

func writeRecord(w recordWriter, offset int64, encoded []byte) error {
	n, err := w.Write(encoded)

	if err == nil && n == len(encoded) {
		return nil
	}

	if err == nil {
		err = errors.New("incomplete record write")
	}

	if truncateErr := w.Truncate(offset); truncateErr != nil {
		return errors.Join(
			ErrRollbackFailed,
			fmt.Errorf("write record: %w", err),
			fmt.Errorf("rollback incomplete write: %w", truncateErr),
		)
	}

	return fmt.Errorf("write record: %w", err)
}

func Open(path string) (*Store, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve storage path: %w", err)
	}

	file, err := os.OpenFile(
		absPath,
		os.O_CREATE|os.O_RDWR|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, err
	}

	store := &Store{
		file:  file,
		path:  absPath,
		index: make(map[string]Entry),
	}

	if err := store.rebuildIndex(); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("rebuild storage index: %w", err)
	}

	return store, nil
}

func (s *Store) Put(key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}
	if s.failed {
		return ErrStoreFailed
	}

	encoded, err := Encode(Record{
		Key:  key,
		Data: data,
	})
	if err != nil {
		return err
	}

	offset, err := s.file.Seek(0, os.SEEK_END)
	if err != nil {
		s.failed = true
		return fmt.Errorf("seek storage before put: %w", err)
	}

	if err := writeRecord(s.file, offset, encoded); err != nil {
		if errors.Is(err, ErrRollbackFailed) {
			s.failed = true
		}
		return err
	}

	if err := s.file.Sync(); err != nil {
		s.failed = true
		return fmt.Errorf("sync put record: %w", err)
	}

	s.index[key] = Entry{
		Offset: offset,
		Size:   len(encoded),
	}

	return nil
}

func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}
	if s.failed {
		return ErrStoreFailed
	}

	if _, ok := s.index[key]; !ok {
		return ErrNotFound
	}

	encoded, err := Encode(Record{
		Type: RecordDelete,
		Key:  key,
	})
	if err != nil {
		return err
	}

	offset, err := s.file.Seek(0, os.SEEK_END)
	if err != nil {
		s.failed = true
		return fmt.Errorf("seek storage before delete: %w", err)
	}

	if err := writeRecord(s.file, offset, encoded); err != nil {
		if errors.Is(err, ErrRollbackFailed) {
			s.failed = true
		}
		return err
	}

	if err := s.file.Sync(); err != nil {
		s.failed = true
		return fmt.Errorf("sync delete record: %w", err)
	}

	delete(s.index, key)
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	return s.file.Close()
}

func (s *Store) Get(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, ErrStoreClosed
	}
	if s.failed {
		return nil, ErrStoreFailed
	}

	entry, ok := s.index[key]
	if !ok {
		return nil, ErrNotFound
	}

	if entry.Size <= 0 {
		s.failed = true
		return nil, fmt.Errorf(
			"invalid record size for key %q: %w",
			key,
			ErrInvalidRecord,
		)
	}

	encoded := make([]byte, entry.Size)

	n, err := s.file.ReadAt(encoded, entry.Offset)
	if err != nil {
		s.failed = true
		return nil, fmt.Errorf("read record for key %q: %w", key, err)
	}
	if n != len(encoded) {
		s.failed = true
		return nil, fmt.Errorf(
			"read record for key %q: %w",
			key,
			errors.New("incomplete record read"),
		)
	}

	record, err := Decode(encoded)
	if err != nil {
		s.failed = true
		return nil, fmt.Errorf("decode record for key %q: %w", key, err)
	}

	if record.Type != RecordPut || record.Key != key {
		s.failed = true
		return nil, fmt.Errorf(
			"index points to an unexpected record for key %q: %w",
			key,
			ErrInvalidRecord,
		)
	}

	return record.Data, nil
}

func (s *Store) rebuildIndex() error {
	info, err := s.file.Stat()
	if err != nil {
		return err
	}

	fileSize := info.Size()
	var offset int64

	for offset < fileSize {
		remaining := fileSize - offset
		if remaining < 6 {
			return s.file.Truncate(offset)
		}

		var prefix [6]byte
		if _, err := s.file.ReadAt(prefix[:], offset); err != nil {
			return err
		}

		gotMagic := binary.BigEndian.Uint32(prefix[0:4])
		gotVersion := binary.BigEndian.Uint16(prefix[4:6])

		if gotMagic != magic {
			return fmt.Errorf(
				"invalid record header at offset %d: %w",
				offset,
				ErrInvalidRecord,
			)
		}

		var currentHeaderSize int
		switch gotVersion {
		case versionV1:
			currentHeaderSize = headerSizeV1
		case versionV2:
			currentHeaderSize = headerSizeV2
		case versionV3:
			currentHeaderSize = headerSizeV3
		default:
			return fmt.Errorf(
				"unsupported record version at offset %d: %w",
				offset,
				ErrInvalidRecord,
			)
		}

		if remaining < int64(currentHeaderSize) {
			return s.file.Truncate(offset)
		}

		header := make([]byte, currentHeaderSize)
		if _, err := s.file.ReadAt(header, offset); err != nil {
			return err
		}

		var keySize uint32
		var dataSize uint64

		if gotVersion == versionV1 {
			keySize = binary.BigEndian.Uint32(header[6:10])
			dataSize = binary.BigEndian.Uint64(header[10:18])
		} else {
			keySize = binary.BigEndian.Uint32(header[7:11])
			dataSize = binary.BigEndian.Uint64(header[11:19])
		}

		if keySize == 0 || keySize > maxKeySize || dataSize > maxDataSize {
			return fmt.Errorf(
				"invalid record size at offset %d: %w",
				offset,
				ErrInvalidRecord,
			)
		}

		recordSize := int64(currentHeaderSize) + int64(keySize) + int64(dataSize)
		if recordSize > remaining {
			return s.file.Truncate(offset)
		}

		encoded := make([]byte, int(recordSize))
		if _, err := s.file.ReadAt(encoded, offset); err != nil {
			return err
		}

		record, err := Decode(encoded)
		if err != nil {
			return fmt.Errorf("decode record at offset %d: %w", offset, err)
		}

		switch record.Type {
		case RecordPut:
			s.index[record.Key] = Entry{
				Offset: offset,
				Size:   int(recordSize),
			}
		case RecordDelete:
			delete(s.index, record.Key)
		default:
			return fmt.Errorf(
				"unknown record type at offset %d: %w",
				offset,
				ErrInvalidRecord,
			)
		}

		offset += recordSize
	}

	return nil
}
