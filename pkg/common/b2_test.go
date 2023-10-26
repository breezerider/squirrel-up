package common

import (
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

// Define a mock struct for S3 Client.
type mockS3Client struct {
	s3iface.S3API
}

func (m *mockS3Client) HeadObject(input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
	switch *input.Key {
	case "valid/key":
		var length int64 = 0
		var modified time.Time = time.Unix(0, 0).UTC()
		return &s3.HeadObjectOutput{
			ContentLength: &length,
			LastModified:  &modified,
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
	case "invalid/bucket":
		return &s3.HeadObjectOutput{}, awserr.New(s3.ErrCodeNoSuchBucket, "", nil)
	case "invalid/key/size":
		var length int64 = -1
		var modified time.Time = time.Unix(1, 0).UTC()
		return &s3.HeadObjectOutput{
			ContentLength: &length,
			LastModified:  &modified,
		}, nil
	}
	return nil, nil
}

func (m *mockS3Client) ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
	switch *input.Prefix {
	case "valid/prefix":
		var key1 string = "valid/prefix/key1"
		var length1 int64 = 1
		var modified1 time.Time = time.Unix(1, 0).UTC()

		var key2 string = "valid/prefix/key2"
		var length2 int64 = 2
		var modified2 time.Time = time.Unix(2, 0).UTC()

		var obj1 s3.Object = s3.Object{
			LastModified: &modified1,
			Key:          &key1,
			Size:         &length1,
		}
		var obj2 s3.Object = s3.Object{
			LastModified: &modified2,
			Key:          &key2,
			Size:         &length2,
		}
		return &s3.ListObjectsV2Output{
			Contents: []*s3.Object{&obj1, &obj2},
		}, nil
	case "invalid/prefix":
		return &s3.ListObjectsV2Output{}, awserr.New("NotFound", "", nil)
	}
	return nil, nil
}

func (m *mockS3Client) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	switch *input.Key {
	case "valid/new/key":
		return &s3.PutObjectOutput{}, nil
	case "invalid/new/key":
		return &s3.PutObjectOutput{}, awserr.New("NotFound", "", nil)
	}
	return nil, nil
}

func (m *mockS3Client) DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
	switch *input.Key {
	case "valid/key":
		return &s3.DeleteObjectOutput{}, nil
	case "invalid/key":
		return &s3.DeleteObjectOutput{}, awserr.New("NotFound", "", nil)
	}
	return nil, nil
}

/* test cases for CreateB2Backend */
func TestCreateB2Backend(t *testing.T) {
	cfg := new(Config)
	if cfg == nil {
		t.Fatalf("could not allocate memory")
	}
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
	}
}

func TestB2GetFileInfoInvalidKey(t *testing.T) {
	// Setup Test
	mockB2 := &B2Backend{
		&mockS3Client{},
	}
	mockURI, err := url.ParseRequestURI("b2://test-bucket/invalid/key")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := mockB2.GetFileInfo(mockURI)

	if fileinfo != nil || err == nil {
		t.Fatalf("unexpected test result: %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, ErrFileNotFound, err.Error(), "err.Error")
	}
}

func TestB2GetFileInfoInvalidKeySize(t *testing.T) {
	// Setup Test
	mockB2 := &B2Backend{
		&mockS3Client{},
	}
	mockURI, err := url.ParseRequestURI("b2://test-bucket/invalid/key/size")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := mockB2.GetFileInfo(mockURI)

	if fileinfo != nil || err == nil {
		t.Fatalf("unexpected test result: %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, "invalid file info", err.Error(), "err.Error")
	}
}

/* test cases for B2Backend.ListFiles */
func TestB2ListFilesValidPrefix(t *testing.T) {
	// Setup Test
	mockB2 := &B2Backend{
		&mockS3Client{},
	}
	mockURI, err := url.ParseRequestURI("b2://test-bucket/valid/prefix")
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

		assertEquals(t, "valid/prefix/key2", fileinfo[1].name, "fileinfo[1].name")
		assertEquals(t, uint64(2), fileinfo[1].size, "fileinfo[1].size")
		assertEquals(t, time.Unix(2, 0).UTC(), fileinfo[1].modified, "fileinfo[1].modified")
	}
}

func TestB2ListFilesInvalidPrefix(t *testing.T) {
	// Setup Test
	mockB2 := &B2Backend{
		&mockS3Client{},
	}
	mockURI, err := url.ParseRequestURI("b2://test-bucket/invalid/prefix")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	fileinfo, err := mockB2.ListFiles(mockURI)

	if fileinfo != nil || err == nil {
		t.Fatalf("unexpected test result: %+v, %+v", fileinfo, err)
	} else {
		assertEquals(t, ErrFileNotFound, err.Error(), "err.Error")
	}
}

/* test cases for B2Backend.StoreFile */
func TestB2StoreFileValidKey(t *testing.T) {
	// Setup Test
	mockB2 := &B2Backend{
		&mockS3Client{},
	}
	mockURI, err := url.ParseRequestURI("b2://test-bucket/valid/new/key")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	err = mockB2.StoreFile(nil, mockURI)

	if err != nil {
		t.Fatalf("unexpected test result: %+v", err)
	}
}

func TestB2StoreFileInvalidKey(t *testing.T) {
	// Setup Test
	mockB2 := &B2Backend{
		&mockS3Client{},
	}
	mockURI, err := url.ParseRequestURI("b2://test-bucket/invalid/new/key")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	err = mockB2.StoreFile(nil, mockURI)

	if err == nil {
		t.Fatalf("unexpected test result: no error returned")
	} else {
		assertEquals(t, ErrFileNotFound, err.Error(), "err.Error")
	}
}

/* test cases for B2Backend.RemoveFile */
func TestB2RemoveFileValidKey(t *testing.T) {
	// Setup Test
	mockB2 := &B2Backend{
		&mockS3Client{},
	}
	mockURI, err := url.ParseRequestURI("b2://test-bucket/valid/key")
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
	mockB2 := &B2Backend{
		&mockS3Client{},
	}
	mockURI, err := url.ParseRequestURI("b2://test-bucket/invalid/key")
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Perform the test
	err = mockB2.RemoveFile(mockURI)

	if err == nil {
		t.Fatalf("unexpected test result: no error returned")
	} else {
		assertEquals(t, ErrFileNotFound, err.Error(), "err.Error")
	}
}
