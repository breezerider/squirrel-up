package common

import (
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

type (
	// FileInfo contains file information.
	FileInfo struct {
		name     string
		size     uint64
		modified time.Time
		isfile   bool
	}

	// StorageBackend is a generic interface to storage backends.
	// Currently, it provisions following methods:
	//   * GetFileInfo to get file information in FileInfo struct.
	//   * ListFiles to list files under a given URI.
	//   * StoreFile to store data to a given URI.
	//   * RemoveFile to remove files under a given URI.
	StorageBackend interface {
		GetFileInfo(uri *url.URL) (*FileInfo, error)
		ListFiles(*url.URL) ([]FileInfo, error)
		StoreFile(io.ReaderAt, int64, *url.URL) error
		RemoveFile(*url.URL) error
	}

	// ProgressReporterFacade is a generic interface to setting up progress reporting:
	// enable and disable progress reporting and set progressbar settings.
	ProgressReporterFacade interface {
		GetProgressEnabled() bool
		SetProgressEnabled(bool)
		GetProgressbarOptions() []progressbar.Option
		SetProgressbarOptions(options ...progressbar.Option)
	}

	// DummyBackend defines a dummy backend.
	DummyBackend struct {
		dummyFiles []FileInfo
		dummyError error
	}
)

// Common error definitions.
const (
	ErrFileNotFound     = "file not found"
	ErrAccessDenied     = "access denied"
	ErrInvalidConfig    = "invalid backend configuration"
	ErrOperationTimeout = "operation timeout"
)

// CreateDummyBackend function that returns a pre-initialized DummyBackend.
var CreateDummyBackend func(cfg *Config) StorageBackend = nil

// CreateStorageBackend is a StorageBackend factory function.
func CreateStorageBackend(uri *url.URL, cfg *Config) (StorageBackend, error) {
	switch uri.Scheme {
	case "dummy":
		if CreateDummyBackend != nil {
			return CreateDummyBackend(cfg), nil
		} else {
			return &DummyBackend{}, nil
		}
	case "b2":
		return CreateB2Backend(cfg), nil
	default:
		return nil, fmt.Errorf("unknown URL scheme %s", uri.Scheme)
	}
}

// Name returns name of the file object.
func (fi *FileInfo) Name() string {
	return fi.name
}

// Size returns size of the file object.
func (fi *FileInfo) Size() uint64 {
	return fi.size
}

// Modified returns last modified date of the file object.
func (fi *FileInfo) Modified() time.Time {
	return fi.modified
}

// Modified returns last modified date of the file object.
func (fi *FileInfo) IsFile() bool {
	return fi.isfile
}

// GenerateDummyFiles generate dummy file info list.
func (d *DummyBackend) GenerateDummyFiles(path string, number uint64) {
	d.dummyFiles = make([]FileInfo, number)
	// fmt.Printf("GenerateDummyFiles: %s, %d\n", path, number)
	for index := range d.dummyFiles {
		d.dummyFiles[index].name = path + string(rune(int('A')+index))
		d.dummyFiles[index].size = uint64(index)
		d.dummyFiles[index].modified = time.Unix(int64(index), 0).UTC()
		d.dummyFiles[index].isfile = true
		// fmt.Printf("item %d: %v\n", index, dummyFiles[index])
	}
}

// GetDummyFiles get dummy file info list.
func (d *DummyBackend) GetDummyFiles() []FileInfo {
	return d.dummyFiles
}

// SetDummyError set dummy error.
func (d *DummyBackend) SetDummyError(err error) {
	d.dummyError = err
}

// GetDummyError get dummy error.
func (d *DummyBackend) GetDummyError() error {
	return d.dummyError
}

// GetFileInfo returns a FileInfo struct filled with information
// about object defined by the input URI.
// Input URI must follow the pattern: dummy://path/to/file.
func (d *DummyBackend) GetFileInfo(uri *url.URL) (*FileInfo, error) {
	var path string = uri.Host + uri.Path

	return &FileInfo{
		name:     path,
		size:     uint64(0),
		modified: time.Unix(0, 0).UTC(),
		isfile:   !strings.HasSuffix(uri.Path, "/"),
	}, d.dummyError
}

// ListFiles return an array of FileInfo structs filled with information
// about objects defined by the input URI.
// Input URI must follow the pattern: dummy://path/to/dir.
func (d *DummyBackend) ListFiles(uri *url.URL) ([]FileInfo, error) {
	return d.dummyFiles, d.dummyError
}

// StoreFile writes a data from `input` to output URI.
// Output URI must follow the pattern: dummy://path/to/file.
func (d *DummyBackend) StoreFile(input io.ReaderAt, length int64, uri *url.URL) error {
	return d.dummyError
}

// RemoveFile remove objects defined by the input URI.
// Input URI must follow the pattern: dummy://path/to/dir.
func (d *DummyBackend) RemoveFile(uri *url.URL) error {
	return d.dummyError
}
