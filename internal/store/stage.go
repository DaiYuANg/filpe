package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

type stagedObject struct {
	file *os.File
	hash string
	size int64
}

func stageObject(reader io.Reader) (*stagedObject, error) {
	if reader == nil {
		return nil, errors.New("object reader is required")
	}
	file, err := os.CreateTemp("", "maxio-put-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	if err := writeStagedObject(file, reader); err != nil {
		if cleanupErr := closeAndRemove(file); cleanupErr != nil {
			return nil, fmt.Errorf("%w; cleanup temp file: %w", err, cleanupErr)
		}
		return nil, err
	}
	return newStagedObject(file)
}

func writeStagedObject(file *os.File, reader io.Reader) error {
	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	return nil
}

func newStagedObject(file *os.File) (*stagedObject, error) {
	size, err := fileSize(file)
	if err != nil {
		if cleanupErr := closeAndRemove(file); cleanupErr != nil {
			return nil, fmt.Errorf("%w; cleanup temp file: %w", err, cleanupErr)
		}
		return nil, err
	}
	hash, err := hashFile(file)
	if err != nil {
		if cleanupErr := closeAndRemove(file); cleanupErr != nil {
			return nil, fmt.Errorf("%w; cleanup temp file: %w", err, cleanupErr)
		}
		return nil, err
	}
	return &stagedObject{
		file: file,
		hash: hash,
		size: size,
	}, nil
}

func fileSize(file *os.File) (int64, error) {
	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat staged object: %w", err)
	}
	return info.Size(), nil
}

func hashFile(file *os.File) (string, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek staged object: %w", err)
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("hash staged object: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (s *stagedObject) Reader() (io.Reader, error) {
	if s == nil || s.file == nil {
		return nil, errors.New("staged object is not available")
	}
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek staged object: %w", err)
	}
	return s.file, nil
}

func (s *stagedObject) Close() error {
	if s == nil || s.file == nil {
		return nil
	}
	return closeAndRemove(s.file)
}

func closeStagedObject(staged *stagedObject) {
	if err := staged.Close(); err != nil {
		_ = err.Error()
	}
}

func closeAndRemove(file *os.File) error {
	path := file.Name()
	err := file.Close()
	if removeErr := os.Remove(path); removeErr != nil {
		err = errors.Join(err, fmt.Errorf("remove temp file: %w", removeErr))
	}
	if err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	return nil
}
