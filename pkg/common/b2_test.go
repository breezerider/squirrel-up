package common

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"

	"github.com/schollz/progressbar/v3"
)

// Define a mock struct for S3 Client.
type (
	mockS3Client struct {
		s3iface.S3API
	}

	mockB2ObjectInfo struct {
		ContentLength int64
		LastModified  time.Time
		VersionId     string
	}

	mockB2KeyInfo struct {
		Key          string
		Size         int64
		LastModified time.Time
	}
)

var (
	expected_keys = map[string]mockB2ObjectInfo{
		"valid/key": {
			ContentLength: 0,
			LastModified:  time.Unix(0, 0).UTC(),
			VersionId:     "valid-key-version",
		},
		"invalid/key/size": {
			ContentLength: -1,
			LastModified:  time.Unix(1, 0).UTC(),
			VersionId:     "invalid-key-size-version",
		},
		"valid/undeleteable/key": {
			ContentLength: 0,
			LastModified:  time.Unix(2, 0).UTC(),
			VersionId:     "valid-undeletable-key-version",
		},
	}

	expected_prefixes = map[string][]mockB2KeyInfo{
		"valid/prefix/": {
			{
				Key:          "valid/prefix/key1",
				Size:         1,
				LastModified: time.Unix(1, 0).UTC(),
			},
			{
				Key:          "valid/prefix/key2",
				Size:         2,
				LastModified: time.Unix(2, 0).UTC(),
			},
		},
	}
)

func (m *mockS3Client) HeadObject(input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
	switch *input.Key {
	case "valid/key", "valid/deletable/key", "valid/undeletable/key", "invalid/key/size":
		mockInfo := expected_keys[*input.Key]
		return &s3.HeadObjectOutput{
			ContentLength: &mockInfo.ContentLength,
			LastModified:  &mockInfo.LastModified,
			VersionId:     &mockInfo.VersionId,
		}, nil
	case "access/denied":
		return nil, awserr.New("AccessDenied", "", nil)
	case "missing/region":
		return nil, awserr.New("MissingRegion", "", nil)
	case "empty/static/creds":
		return nil, awserr.New("EmptyStaticCreds", "", nil)
	case "not/found":
		return &s3.HeadObjectOutput{}, awserr.New("NotFound", "", nil)
	case "invalid/key":
		return &s3.HeadObjectOutput{}, awserr.New(s3.ErrCodeNoSuchKey, "", nil)
	}
	return nil, fmt.Errorf("mockS3Client.HeadObject got an unexpected key %s", *input.Key)
}

func (m *mockS3Client) ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
	switch *input.Prefix {
	case "valid/prefix/":
		var contents = make([]*s3.Object, len(expected_prefixes[*input.Prefix]))

		for index := range expected_prefixes[*input.Prefix] {
			s3Object := new(s3.Object)
			s3Object.Key = &expected_prefixes[*input.Prefix][index].Key
			s3Object.Size = &expected_prefixes[*input.Prefix][index].Size
			s3Object.LastModified = &expected_prefixes[*input.Prefix][index].LastModified

			contents[index] = s3Object
		}

		return &s3.ListObjectsV2Output{Contents: contents}, nil
	case "invalid/prefix/":
		return &s3.ListObjectsV2Output{}, awserr.New("NotFound", "", nil)
	}
	return nil, fmt.Errorf("mockS3Client.ListObjectsV2 got an unexpected prefix %s", *input.Prefix)
}

func (m *mockS3Client) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	switch *input.Key {
	case "valid/new/key":
		_, _ = input.Body.Seek(0, 0)
		_, err := io.ReadAll(input.Body)
		return &s3.PutObjectOutput{}, err
	case "invalid/new/key":
		return &s3.PutObjectOutput{}, awserr.New("NotFound", "", nil)
	}
	return nil, fmt.Errorf("mockS3Client.PutObject got an unexpected key %s", *input.Key)
}

func (m *mockS3Client) DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
	switch *input.Key {
	case "valid/deletable/key":
		return &s3.DeleteObjectOutput{}, nil
	case "valid/undeletable/key":
		return &s3.DeleteObjectOutput{}, awserr.New("AccessDenied", "", nil)
	}
	return nil, fmt.Errorf("mockS3Client.DeleteObject got an unexpected key %s", *input.Key)
}

func setupB2Backend() *B2Backend {
	return &B2Backend{
		&mockS3Client{},
		false,
		[]progressbar.Option{},
	}
}

/* test cases for CreateB2Backend */
func TestCreateB2Backend(t *testing.T) {
	cfg := new(Config)
	cfg.S3.Region = "mock-region"
	cfg.S3.ID = "mock-id"
	cfg.S3.Secret = "mock-secret"
	cfg.S3.Token = "mock-token"

	checkS3Client = func(s3Client *s3.S3) {
		assertEquals(t, "mock-region", *s3Client.Config.Region, "aws.Config.Region")
		assertEquals(t, "https://s3.mock-region.backblazeb2.com", *s3Client.Config.Endpoint, "aws.Config.Endpoint")
		// assertEquals(t, "mock-id", *s3Client.Config.Credentials.AccessKeyID, "aws.Config.Credentials.AccessKeyID")
		// assertEquals(t, "mock-secret", *s3Client.Config.Credentials.SecretAccessKey, "aws.Config.Credentials.SecretAccessKey")
		// assertEquals(t, "mock-token", *s3Client.Config.Credentials.SessionToken, "aws.Config.Credentials.SessionToken")
	}

	_ = CreateB2Backend(cfg)
}

/* test cases for handleError */
func TestB2HandleErrorAWSError(t *testing.T) {
	tests := map[string]string{
		"NotFound":             ErrFileNotFound,
		s3.ErrCodeNoSuchBucket: ErrFileNotFound,
		s3.ErrCodeNoSuchKey:    ErrFileNotFound,
		"AccessDenied":         ErrAccessDenied,
		"MissingRegion":        ErrInvalidConfig,
		"EmptyStaticCreds":     ErrInvalidConfig,
		"UnknownError":         "unknown B2 error (UnknownError: ).",
	}

	// Iterate over all keys in a sorted order
	for key, val := range tests {
		t.Run(key, func(t *testing.T) {
			assertEquals(t, val, handleError(awserr.New(key, "", nil)).Error(), "err.Error")
		})
	}
}

/* test cases for B2Backend.GetFileInfo */
func TestB2GetFileInfoValidKey(t *testing.T) {
	// Setup Test
	mockB2 := &B2Backend{
		&mockS3Client{},
		false,
		[]progressbar.Option{},
	}
	mockURI, err := url.ParseRequestURI("b2://test-bucket/valid/key")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := mockB2.GetFileInfo(mockURI)

	if fileinfo == nil || err != nil {
		t.Fatalf("unexpected test result: %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, "valid/key", fileinfo.name, "fileinfo.name")
		assertEquals(t, uint64(0), fileinfo.size, "fileinfo.size")
		assertEquals(t, time.Unix(0, 0).UTC(), fileinfo.modified, "fileinfo.modified")
		assertEquals(t, true, fileinfo.isfile, "fileinfo.isfile")
	}
}

func TestB2GetFileInfoValidPrefix(t *testing.T) {
	// Setup Test
	mockB2 := setupB2Backend()
	mockURI, err := url.ParseRequestURI("b2://test-bucket/valid/prefix/")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := mockB2.GetFileInfo(mockURI)

	if fileinfo == nil || err != nil {
		t.Fatalf("unexpected test result: %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, "valid/prefix/", fileinfo.name, "fileinfo.name")
		assertEquals(t, uint64(3), fileinfo.size, "fileinfo.size")
		assertEquals(t, time.Unix(2, 0).UTC(), fileinfo.modified, "fileinfo.modified")
		assertEquals(t, false, fileinfo.isfile, "fileinfo.isfile")
	}
}

func TestB2GetFileInfoInvalidKey(t *testing.T) {
	// Setup Test
	mockB2 := setupB2Backend()
	mockURI, err := url.ParseRequestURI("b2://test-bucket/invalid/key")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := mockB2.GetFileInfo(mockURI)

	if fileinfo != nil || err == nil {
		t.Fatalf("unexpected test result: GetFileInfo was supposed to fail, but instead returned %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, ErrFileNotFound, err.Error(), "err.Error")
	}
}

func TestB2GetFileInfoInvalidPrefix(t *testing.T) {
	// Setup Test
	mockB2 := setupB2Backend()
	mockURI, err := url.ParseRequestURI("b2://test-bucket/invalid/prefix/")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := mockB2.GetFileInfo(mockURI)

	if fileinfo != nil || err == nil {
		t.Fatalf("unexpected test result: GetFileInfo was supposed to fail, but instead returned %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, ErrFileNotFound, err.Error(), "err.Error")
	}
}

func TestB2GetFileInfoInvalidKeySize(t *testing.T) {
	// Setup Test
	mockB2 := setupB2Backend()
	mockURI, err := url.ParseRequestURI("b2://test-bucket/invalid/key/size")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := mockB2.GetFileInfo(mockURI)

	if fileinfo != nil || err == nil {
		t.Fatalf("unexpected test result: GetFileInfo was supposed to fail, but instead returned %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, "invalid file info", err.Error(), "err.Error")
	}
}

/* test cases for B2Backend.ListFiles */
func TestB2ListFilesValidPrefix(t *testing.T) {
	// Setup Test
	mockB2 := setupB2Backend()
	mockURI, err := url.ParseRequestURI("b2://test-bucket/valid/prefix/")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := mockB2.ListFiles(mockURI)

	if fileinfo == nil || err != nil {
		t.Fatalf("unexpected test result: %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, 2, len(fileinfo), "len(fileinfo)")

		assertEquals(t, "valid/prefix/key1", fileinfo[0].name, "fileinfo[0].name")
		assertEquals(t, uint64(1), fileinfo[0].size, "fileinfo[0].size")
		assertEquals(t, time.Unix(1, 0).UTC(), fileinfo[0].modified, "fileinfo[0].modified")
		assertEquals(t, true, fileinfo[0].isfile, "fileinfo[0].isfile")

		assertEquals(t, "valid/prefix/key2", fileinfo[1].name, "fileinfo[1].name")
		assertEquals(t, uint64(2), fileinfo[1].size, "fileinfo[1].size")
		assertEquals(t, time.Unix(2, 0).UTC(), fileinfo[1].modified, "fileinfo[1].modified")
		assertEquals(t, true, fileinfo[1].isfile, "fileinfo[1].isfile")
	}
}

func TestB2ListFilesInvalidPrefix(t *testing.T) {
	// Setup Test
	mockB2 := setupB2Backend()
	mockURI, err := url.ParseRequestURI("b2://test-bucket/invalid/prefix/")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := mockB2.ListFiles(mockURI)

	if fileinfo != nil || err == nil {
		t.Fatalf("unexpected test result: ListFiles was supposed to fail, but instead returned %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, ErrFileNotFound, err.Error(), "err.Error")
	}
}

/* test cases for B2Backend.StoreFile */
func TestB2StoreFileValidKey(t *testing.T) {
	// Setup Test
	mockB2 := setupB2Backend()
	mockURI, err := url.ParseRequestURI("b2://test-bucket/valid/new/key")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	data := []byte("test")
	err = mockB2.StoreFile(bytes.NewReader(data), mockURI)

	if err != nil {
		t.Fatalf("unexpected test result: %+v", err)
	}
}

func TestB2StoreFileInvalidKey(t *testing.T) {
	// Setup Test
	mockB2 := setupB2Backend()
	mockURI, err := url.ParseRequestURI("b2://test-bucket/invalid/new/key")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	data := []byte("test")
	err = mockB2.StoreFile(bytes.NewReader(data), mockURI)

	if err == nil {
		t.Fatalf("unexpected test result: StoreFile was supposed to fail")
	} else {
		assertEquals(t, ErrFileNotFound, err.Error(), "err.Error")
	}
}

/* test cases for B2Backend.RemoveFile */
func TestB2RemoveFileValidKey(t *testing.T) {
	// Setup Test
	mockB2 := setupB2Backend()
	mockURI, err := url.ParseRequestURI("b2://test-bucket/valid/deletable/key")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	err = mockB2.RemoveFile(mockURI)

	if err != nil {
		t.Fatalf("unexpected test result: %+v", err)
	}
}

func TestB2RemoveFileInvalidKey(t *testing.T) {
	// Setup Test
	mockB2 := setupB2Backend()
	mockURI, err := url.ParseRequestURI("b2://test-bucket/invalid/key")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	err = mockB2.RemoveFile(mockURI)

	if err == nil {
		t.Fatalf("unexpected test result: RemoveFile was supposed to fail")
	} else {
		assertEquals(t, ErrFileNotFound, err.Error(), "err.Error")
	}
}

func TestB2RemoveFileUndeletableKey(t *testing.T) {
	// Setup Test
	mockB2 := setupB2Backend()
	mockURI, err := url.ParseRequestURI("b2://test-bucket/valid/undeletable/key")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	err = mockB2.RemoveFile(mockURI)

	if err == nil {
		t.Fatalf("unexpected test result: RemoveFile was supposed to fail")
	} else {
		assertEquals(t, ErrAccessDenied, err.Error(), "err.Error")
	}
}
