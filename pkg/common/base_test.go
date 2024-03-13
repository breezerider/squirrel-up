package common

import (
	"fmt"
	"net/url"
	"testing"
	"time"
)

func assertEquals(t *testing.T, expected any, actual any, description string) {
	if actual != expected {
		t.Fatalf("assertion %s failed:\nexpected: %+v\nactual: %+v\n", description, expected, actual)
	}
}

/* test cases for FileInfo */
func TestFileInfo(t *testing.T) {
	fileinfo := FileInfo{
		name:     "path/to/file",
		size:     uint64(0),
		modified: time.Unix(0, 0).UTC(),
		isfile:   true,
	}

	assertEquals(t, "path/to/file", fileinfo.Name(), "fileinfo.Name")
	assertEquals(t, uint64(0), fileinfo.Size(), "fileinfo.Size")
	assertEquals(t, time.Unix(0, 0).UTC(), fileinfo.Modified(), "fileinfo.Modified")
	assertEquals(t, true, fileinfo.IsFile(), "fileinfo.IsFile")

}

/* test cases for CreateStorageBackend */
func TestCreateStorageBackendDummy(t *testing.T) {
	cfg := new(Config)

	mockURI, err := url.ParseRequestURI("dummy://path/to/file")
	if err != nil {
		t.Fatalf(err.Error())
	}

	backend, err := CreateStorageBackend(mockURI, cfg)
	if err != nil {
		t.Fatalf(err.Error())
	} else if backend == nil {
		t.Fatalf("failed to create a DummyBackend")
	}
}

func TestCreateStorageBackendDummyCustom(t *testing.T) {
	cfg := new(Config)

	mockURI, err := url.ParseRequestURI("dummy://path/to/file")
	if err != nil {
		t.Fatalf(err.Error())
	}

	CreateDummyBackend = func(cfg *Config) StorageBackend {
		return &DummyBackend{}
	}

	backend, err := CreateStorageBackend(mockURI, cfg)
	if err != nil {
		t.Fatalf(err.Error())
	} else if backend == nil {
		t.Fatalf("failed to create a DummyBackend")
	}
}

func TestCreateStorageBackendB2(t *testing.T) {
	cfg := new(Config)
	cfg.S3.Region = "mock-region"
	cfg.S3.ID = "mock-id"
	cfg.S3.Secret = "mock-secret"
	cfg.S3.Token = "mock-token"

	mockURI, err := url.ParseRequestURI("b2://bucket/path/to/key")
	if err != nil {
		t.Fatalf(err.Error())
	}

	backend, err := CreateStorageBackend(mockURI, cfg)
	if err != nil {
		t.Fatalf(err.Error())
	} else if backend == nil {
		t.Fatalf("failed to create a B2Backend")
	}
}

func TestCreateStorageBackendUnknown(t *testing.T) {
	cfg := new(Config)

	mockURI, err := url.ParseRequestURI("unknown://test/path/")
	if err != nil {
		t.Fatalf(err.Error())
	}

	backend, err := CreateStorageBackend(mockURI, cfg)
	if err == nil {
		t.Fatalf("storage backend creation expected to fail")
	} else {
		assertEquals(t, "unknown URL scheme unknown", err.Error(), "err.Error")
		assertEquals(t, nil, backend, "backend")
	}
}

/* test cases for DummyBackend.GetFileInfo */
func TestDummyGetFileInfoFile(t *testing.T) {
	// Setup Test
	dummy := &DummyBackend{}
	dummyErr := fmt.Errorf("dummy error")
	dummy.SetDummyError(dummyErr)

	mockURI, err := url.ParseRequestURI("dummy://path/to/file")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := dummy.GetFileInfo(mockURI)

	if fileinfo == nil {
		t.Fatalf("unexpected test result: %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, err, dummy.GetDummyError(), "dummyError")

		assertEquals(t, "path/to/file", fileinfo.name, "fileinfo.name")
		assertEquals(t, uint64(0), fileinfo.size, "fileinfo.size")
		assertEquals(t, time.Unix(0, 0).UTC(), fileinfo.modified, "fileinfo.modified")
		assertEquals(t, true, fileinfo.isfile, "fileinfo.isfile")
	}
}

func TestDummyGetFileInfoDir(t *testing.T) {
	// Setup Test
	dummy := &DummyBackend{}
	dummyErr := fmt.Errorf("dummy error")
	dummy.SetDummyError(dummyErr)

	mockURI, err := url.ParseRequestURI("dummy://path/to/dir/")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := dummy.GetFileInfo(mockURI)

	if fileinfo == nil {
		t.Fatalf("unexpected test result: %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, err, dummy.GetDummyError(), "dummyError")

		assertEquals(t, "path/to/dir/", fileinfo.name, "fileinfo.name")
		assertEquals(t, uint64(0), fileinfo.size, "fileinfo.size")
		assertEquals(t, time.Unix(0, 0).UTC(), fileinfo.modified, "fileinfo.modified")
		assertEquals(t, false, fileinfo.isfile, "fileinfo.isfile")
	}
}

/* test cases for DummyBackend.ListFiles */
func TestDummyListFilesEmpty(t *testing.T) {
	// Setup Test
	dummy := &DummyBackend{}
	dummyErr := fmt.Errorf("dummy error")
	dummy.SetDummyError(dummyErr)

	mockURI, err := url.ParseRequestURI("dummy://path/to/dir")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := dummy.ListFiles(mockURI)

	assertEquals(t, 0, len(fileinfo), "len(fileinfo)")
	assertEquals(t, err, dummy.GetDummyError(), "dummyError")
}

func TestDummyListFilesMock(t *testing.T) {
	// Setup Test
	dummy := &DummyBackend{}
	dummyErr := fmt.Errorf("dummy error")
	dummy.SetDummyError(dummyErr)

	mockURI, err := url.ParseRequestURI("dummy://path/to/dir")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// setup test
	dummy.GenerateDummyFiles("to/dir/", 2)

	// Perform the test
	filelist, err := dummy.ListFiles(mockURI)

	if filelist == nil {
		t.Fatalf("unexpected test result: %+v, %+v", filelist, err)
	} else {
		assertEquals(t, 2, len(filelist), "len(filelist)")
		assertEquals(t, err, dummy.GetDummyError(), "dummyError")

		dummyFiles := dummy.GetDummyFiles()
		for index, fileinfo := range filelist {
			assertEquals(t, dummyFiles[index].Name(), fileinfo.Name(), "fileinfo.name")
			assertEquals(t, dummyFiles[index].Size(), fileinfo.Size(), "fileinfo.size")
			assertEquals(t, dummyFiles[index].Modified(), fileinfo.Modified(), "fileinfo.modified")
			assertEquals(t, dummyFiles[index].IsFile(), fileinfo.IsFile(), "fileinfo.isfile")
		}
	}
}

/* test cases for DummyBackend.StoreFile */
func TestDummyStoreFile(t *testing.T) {
	// Setup Test
	dummy := &DummyBackend{}
	dummyErr := fmt.Errorf("dummy error")
	dummy.SetDummyError(dummyErr)

	mockURI, err := url.ParseRequestURI("dummy://path/to/file")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	err = dummy.StoreFile(nil, mockURI)
	assertEquals(t, err, dummy.GetDummyError(), "dummyError")
}

/* test cases for DummyBackend.RemoveFile */
func TestDummyRemoveFile(t *testing.T) {
	// Setup Test
	dummy := &DummyBackend{}
	dummyErr := fmt.Errorf("dummy error")
	dummy.SetDummyError(dummyErr)

	mockURI, err := url.ParseRequestURI("dummy://path/to/file")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	err = dummy.RemoveFile(mockURI)
	assertEquals(t, err, dummy.GetDummyError(), "dummyError")
}

/* test cases for DummyBackend.dummyError */
func TestDummyError(t *testing.T) {
	// Setup Test
	dummy := &DummyBackend{}

	// Perform the test
	err := fmt.Errorf("dummy error")
	dummy.SetDummyError(err)
	assertEquals(t, err, dummy.GetDummyError(), "dummyError")
}
