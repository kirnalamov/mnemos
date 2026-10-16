package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"
)

var ErrStoreClosed = errors.New("storage is closed")
var ErrNotFound = errors.New("object not found")

type Entry struct {
	Offset int64
	Size   int
}

type Store struct {
	mu     sync.Mutex
	file   *os.File
	index  map[string]Entry
	closed bool
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
			fmt.Errorf("write record: %w", err),
			fmt.Errorf("rollback incomplete write: %w", truncateErr),
		)
	}

	return fmt.Errorf("write record: %w", err)
}
func Open(path string) (*Store, error) {
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_RDWR|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, err
	}

	store := &Store{
		file:  file,
		index: make(map[string]Entry),
	}

	if err := store.rebuildIndex(); err != nil {
		file.Close()
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

	encoded, err := Encode(Record{
		Key:  key,
		Data: data,
	})
	if err != nil {
		return err
	}

	offset, err := s.file.Seek(0, os.SEEK_END)
	if err != nil {
		return err
	}

	if err := writeRecord(s.file, offset, encoded); err != nil {
		return err
	}

	s.index[key] = Entry{
		Offset: offset,
		Size:   len(encoded),
	}

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

	entry, ok := s.index[key]
	if !ok {
		return nil, ErrNotFound
	}

	encoded := make([]byte, entry.Size)

	_, err := s.file.ReadAt(encoded, entry.Offset)
	if err != nil {
		return nil, err
	}

	record, err := Decode(encoded)
	if err != nil {
		return nil, err
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
		if fileSize-offset < int64(headerSize) {
			return s.file.Truncate(offset)
		}

		header := make([]byte, headerSize)
		if _, err := s.file.ReadAt(header, offset); err != nil {
			return err
		}

		gotMagic := binary.BigEndian.Uint32(header[0:4])
		gotVersion := binary.BigEndian.Uint16(header[4:6])
		keySize := binary.BigEndian.Uint32(header[6:10])
		dataSize := binary.BigEndian.Uint64(header[10:18])

		if gotMagic != magic || gotVersion != version {
			return fmt.Errorf(
				"invalid record header at offset %d: %w",
				offset,
				ErrInvalidRecord,
			)
		}

		if keySize == 0 || keySize > maxKeySize || dataSize > maxDataSize {
			return fmt.Errorf(
				"invalid record size at offset %d: %w",
				offset,
				ErrInvalidRecord,
			)
		}

		recordSize := int64(headerSize) + int64(keySize) + int64(dataSize)

		if recordSize > fileSize-offset {
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

		s.index[record.Key] = Entry{
			Offset: offset,
			Size:   int(recordSize),
		}

		offset += recordSize
	}

	return nil
}
