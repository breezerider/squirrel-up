package common

import (
	"net/url"
	"testing"
	"time"
)

func assertEquals(t *testing.T, expected any, actual any, description string) {
	if actual != expected {
		t.Fatalf("assertion %s failed:\nexpected: %+v\nactual: %+v\n", description, expected, actual)
	}
}

/* test cases for CreateStorageBackend */
func TestCreateStorageBackendDummy(t *testing.T) {
	cfg := new(Config)
	if cfg == nil {
		t.Fatalf("could not allocate memory")
	}

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

func TestCreateStorageBackendB2(t *testing.T) {
	cfg := new(Config)
	if cfg == nil {
		t.Fatalf("could not allocate memory")
	}
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
	if cfg == nil {
		t.Fatalf("could not allocate memory")
	}

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
func TestDummyGetFileInfo(t *testing.T) {
	// Setup Test
	dummy := &DummyBackend{}
	mockURI, err := url.ParseRequestURI("dummy://path/to/file")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := dummy.GetFileInfo(mockURI)

	if fileinfo == nil || err != nil {
		t.Fatalf("unexpected test result: %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, "path/to/file", fileinfo.name, "fileinfo.name")
		assertEquals(t, uint64(0), fileinfo.size, "fileinfo.size")
		assertEquals(t, time.Unix(0, 0).UTC(), fileinfo.modified, "fileinfo.modified")
	}
}

/* test cases for DummyBackend.ListFiles */
func TestDummyListFiles(t *testing.T) {
	// Setup Test
	dummy := &DummyBackend{}
	mockURI, err := url.ParseRequestURI("dummy://path/to/dir")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := dummy.ListFiles(mockURI)

	if fileinfo == nil || err != nil {
		t.Fatalf("unexpected test result: %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, 0, len(fileinfo), "len(fileinfo)")
	}
}

/* test cases for DummyBackend.StoreFile */
func TestDummyStoreFile(t *testing.T) {
	// Setup Test
	mockB2 := &DummyBackend{}
	mockURI, err := url.ParseRequestURI("dummy://path/to/file")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	err = mockB2.StoreFile(nil, mockURI)

	if err != nil {
		t.Fatalf("unexpected test result: %+v", err)
	}
}
