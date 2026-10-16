package storage

import (
	"errors"
	"os"
	"sync"
)

var ErrStoreClosed = errors.New("storage is closed")

type Store struct {
	mu     sync.Mutex
	file   *os.File
	closed bool
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

	return &Store{
		file: file,
	}, nil
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

	_, err = s.file.Write(encoded)
	return err
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
