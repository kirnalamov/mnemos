package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

var (
	createCompactTemp    = os.CreateTemp
	renameCompactFile    = os.Rename
	openCompactFile      = os.OpenFile
	syncCompactDirectory = syncDirectory
)

func syncDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}

	syncErr := f.Sync()
	closeErr := f.Close()

	return errors.Join(syncErr, closeErr)
}

func (s *Store) Compact() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}
	if s.failed {
		return ErrStoreFailed
	}
	if s.path == "" {
		return errors.New("storage path is not configured")
	}

	info, err := s.file.Stat()
	if err != nil {
		return fmt.Errorf("stat original storage: %w", err)
	}

	dir := filepath.Dir(s.path)
	tmp, err := createCompactTemp(dir, ".mnemos-compact-*")
	if err != nil {
		return fmt.Errorf("create temporary storage file: %w", err)
	}

	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("preserve storage permissions: %w", err)
	}

	newIndex := make(map[string]Entry, len(s.index))
	offset := int64(0)

	keys := make([]string, 0, len(s.index))
	for key := range s.index {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		entry := s.index[key]
		if entry.Size <= 0 || entry.Offset < 0 {
			_ = tmp.Close()
			return fmt.Errorf("invalid index entry for key %q", key)
		}

		encoded := make([]byte, entry.Size)
		n, err := s.file.ReadAt(encoded, entry.Offset)
		if n != len(encoded) {
			_ = tmp.Close()
			if err == nil {
				err = io.ErrUnexpectedEOF
			}
			return fmt.Errorf("read record %q: %w", key, err)
		}
		if err != nil {
			_ = tmp.Close()
			return fmt.Errorf("read record %q: %w", key, err)
		}

		record, err := Decode(encoded)
		if err != nil {
			_ = tmp.Close()
			return fmt.Errorf("decode record %q: %w", key, err)
		}
		if record.Type != RecordPut || record.Key != key {
			_ = tmp.Close()
			return fmt.Errorf(
				"index mismatch for key %q: %w",
				key,
				ErrInvalidRecord,
			)
		}

		compacted, err := Encode(Record{
			Type: RecordPut,
			Key:  record.Key,
			Data: record.Data,
		})
		if err != nil {
			_ = tmp.Close()
			return fmt.Errorf("encode record %q: %w", key, err)
		}

		n, err = tmp.Write(compacted)
		if err != nil {
			_ = tmp.Close()
			return fmt.Errorf("write record %q: %w", key, err)
		}
		if n != len(compacted) {
			_ = tmp.Close()
			return fmt.Errorf("write record %q: %w", key, io.ErrShortWrite)
		}

		newIndex[key] = Entry{
			Offset: offset,
			Size:   len(compacted),
		}
		offset += int64(len(compacted))
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync compacted storage: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close compacted storage: %w", err)
	}

	// Закрываем исходный файл перед заменой: это также нужно
	// на платформах, не допускающих переименование открытого файла.
	if err := s.file.Close(); err != nil {
		s.failed = true
		return fmt.Errorf("close original storage: %w", err)
	}

	if err := renameCompactFile(tmpPath, s.path); err != nil {
		// Не используем O_CREATE: если исходного файла больше нет,
		// нельзя подменять его новым пустым файлом.
		file, reopenErr := openCompactFile(
			s.path,
			os.O_RDWR|os.O_APPEND,
			info.Mode().Perm(),
		)
		if reopenErr != nil {
			s.failed = true
			return errors.Join(
				fmt.Errorf("replace storage: %w", err),
				fmt.Errorf("reopen original storage: %w", reopenErr),
			)
		}

		s.file = file
		return fmt.Errorf("replace storage: %w", err)
	}
	if err := syncCompactDirectory(dir); err != nil {
		s.failed = true
		return fmt.Errorf("sync storage directory: %w", err)
	}
	file, err := openCompactFile(
		s.path,
		os.O_RDWR|os.O_APPEND,
		info.Mode().Perm(),
	)
	if err != nil {
		s.failed = true
		return fmt.Errorf("reopen compacted storage: %w", err)
	}

	s.file = file
	s.index = newIndex

	return nil
}
