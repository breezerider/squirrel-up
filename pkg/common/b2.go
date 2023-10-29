package common

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

// B2Backend is struct that holds active B2 session.
type B2Backend struct {
	s3iface.S3API
}

var checkS3Client func(*s3.S3)

// CreateB2Backend is the B2Backend factory function.
func CreateB2Backend(cfg *Config) *B2Backend {
	s3Client := s3.New(session.Must(session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(cfg.S3.ID, cfg.S3.Secret, cfg.S3.Token),
		Endpoint:         aws.String(fmt.Sprintf("https://s3.%s.backblazeb2.com", cfg.S3.Region)),
		Region:           aws.String(cfg.S3.Region),
		S3ForcePathStyle: aws.Bool(true),
	})))
	if checkS3Client != nil {
		checkS3Client(s3Client)
	}
	return &B2Backend{
		s3Client,
	}
}

func handleError(err error) error {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case "NotFound":
			fallthrough
		case s3.ErrCodeNoSuchBucket:
			fallthrough
		case s3.ErrCodeNoSuchKey:
			return errors.New(ErrFileNotFound)
		case "AccessDenied":
			return errors.New(ErrAccessDenied)
		case "MissingRegion":
			fallthrough
		case "EmptyStaticCreds":
			return errors.New(ErrInvalidConfig)
		}
	}
	return fmt.Errorf("unknown B2 error (%s).", err.Error())
}

// GetFileInfo returns a FileInfo struct filled with information
// about object defined by the input URI.
// Input URI must follow the pattern: b2://bucket/path/to/key.
func (b2 *B2Backend) GetFileInfo(uri *url.URL) (*FileInfo, error) {
	var bucket string = uri.Host
	var key string = strings.TrimPrefix(uri.Path, "/")

	// get object properties stored in S3 bucket under key
	resp, err := b2.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, handleError(err)
	}

	filesize := *resp.ContentLength
	if filesize < 0 {
		return nil, errors.New("invalid file info")
	}

	return &FileInfo{
		name:     key,
		size:     uint64(filesize),
		modified: *resp.LastModified,
	}, nil
}

// ListFiles return an array of FileInfo structs filled with information
// about objects defined by the input URI.
// Input URI must follow the pattern: b2://bucket/path/to/prefix.
func (b2 *B2Backend) ListFiles(uri *url.URL) ([]FileInfo, error) {
	var bucket string = uri.Host
	var prefix string = strings.TrimPrefix(uri.Path, "/")

	// list object stored under given prefix
	objects, err := b2.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, handleError(err)
	}

	result := make([]FileInfo, len(objects.Contents))
	for index, item := range objects.Contents {
		result[index].name = *item.Key
		result[index].size = uint64(*item.Size)
		result[index].modified = *item.LastModified
	}

	return result, nil
}

// StoreFile writes data from `input` to output URI.
// Output URI must follow the pattern: b2://bucket/path/to/key.
func (b2 *B2Backend) StoreFile(input io.ReadSeekCloser, uri *url.URL) error {
	var bucket string = uri.Host
	var key string = strings.TrimPrefix(uri.Path, "/")

	// upload reader contents to S3 bucket as an object with the given key
	_, err := b2.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   aws.ReadSeekCloser(input),
	})

	if err != nil {
		return handleError(err)
	}
	return nil
}

// RemoveFile removes an object under the given URI.
// Object URI must follow the pattern: b2://bucket/path/to/key.
func (b2 *B2Backend) RemoveFile(uri *url.URL) error {
	var bucket string = uri.Host
	var key string = strings.TrimPrefix(uri.Path, "/")

	// get object properties stored in S3 bucket under key
	resp, err := b2.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return handleError(err)
	}

	// remove object with the given key from S3 bucket
	_, err = b2.DeleteObject(&s3.DeleteObjectInput{
		Bucket:    aws.String(bucket),
		Key:       aws.String(key),
		VersionId: resp.VersionId,
	})

	if err != nil {
		return handleError(err)
	}
	return nil
}
