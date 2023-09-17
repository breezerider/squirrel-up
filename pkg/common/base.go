package common

import (
	"fmt"
	"io"
	"net/url"
	"time"
)

type (
	// FileInfo contains file information.
	FileInfo struct {
		name     string
		size     uint64
		modified time.Time
	}

	// StorageBackend si a generic interface to storage backends.
	// Currently, it provides three functions:
	//   * GetFileInfo to get file information in FileInfo struct.
	//   * ListFiles to list files under a given URI.
	//   * StoreFile to store data to a given URI.
	StorageBackend interface {
		GetFileInfo(uri *url.URL) (*FileInfo, error)
		ListFiles(*url.URL) ([]FileInfo, error)
		StoreFile(io.ReadSeekCloser, *url.URL) error
	}

	// DummyBackend defines a dummy backend.
	DummyBackend struct {
	}
)

// Common error definitions.
const (
	ErrFileNotFound  = "file not found"
	ErrAccessDenied  = "access denied"
	ErrInvalidConfig = "invalid backend configuration"
)

// CreateStorageBackend is a StorageBackend factory function.
func CreateStorageBackend(uri *url.URL, cfg *Config) (StorageBackend, error) {
	switch uri.Scheme {
	case "dummy":
		return &DummyBackend{}, nil
	case "b2":
		return CreateB2Backend(cfg), nil
	default:
		return nil, fmt.Errorf("unknown URL scheme %s", uri.Scheme)
	}
}

// GetFileInfo returns a FileInfo struct filled with information
// about object defined by the input URI.
// Input URI must follow the pattern: dummy://path/to/file.
func (*DummyBackend) GetFileInfo(uri *url.URL) (*FileInfo, error) {
	var path string = uri.Host + uri.Path

	return &FileInfo{
		name:     path,
		size:     uint64(0),
		modified: time.Unix(0, 0).UTC(),
	}, nil
}

// ListFiles return an array of FileInfo structs filled with information
// about objects defined by the input URI.
// Input URI must follow the pattern: dummy://path/to/dir.
func (*DummyBackend) ListFiles(uri *url.URL) ([]FileInfo, error) {
	return []FileInfo{}, nil
}

// StoreFile writes a data from `input` to output URI.
// Output URI must follow the pattern: dummy://path/to/file.
func (*DummyBackend) StoreFile(input io.ReadSeekCloser, uri *url.URL) error {
	return nil
}
