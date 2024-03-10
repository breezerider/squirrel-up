package common

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"

	"github.com/schollz/progressbar/v3"
)

// B2Backend is struct that holds active B2 session.
type (
	B2Backend struct {
		s3iface.S3API
		progressEnabled bool
		options         []progressbar.Option
	}

	progressReadSeeker struct {
		io.ReadSeeker
		bar  *progressbar.ProgressBar
		read int64
		lock sync.Mutex
	}
)

const (
	put_get_object_max_bytes   = 512 * 1024 * 1024
	multipart_upload_part_size = 100 * 1024 * 1024
)

var checkS3Client func(*s3.S3)

// NewReader return a new Reader with a given progress bar.
func newProgressReadSeeker(rs io.ReadSeeker, bar *progressbar.ProgressBar) *progressReadSeeker {
	return &progressReadSeeker{
		ReadSeeker: rs,
		bar:        bar,
		read:       0,
	}
}

// Read will read the data and add the number of bytes to the progressbar for sgnign and uploading.
func (prs *progressReadSeeker) Read(p []byte) (n int, err error) {
	n, err = prs.ReadSeeker.Read(p)

	prs.lock.Lock()
	defer prs.lock.Unlock()

	if prs.bar != nil {
		if prs.read < prs.bar.GetMax64() {
			if prs.read == 0 {
				prs.bar.Describe("signing")
			}
			_ = prs.bar.Add(n)

			if prs.bar.IsFinished() {
				prs.bar.Reset()
				prs.bar.Describe("uploading")
			}
		} else {
			_ = prs.bar.Add(n)
		}
	}

	prs.read += int64(n)

	return
}

// Seek the input stream by forwarding the call.
func (prs *progressReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return prs.ReadSeeker.Seek(offset, whence)
}

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
		false,
		[]progressbar.Option{},
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
	var keysize uint64 = 0
	var modifieddate time.Time
	var isfile bool

	// is this a prefix path?
	if strings.HasSuffix(key, "/") {
		isfile = false

		filelist, err := b2.ListFiles(uri)
		if err != nil {
			return nil, err
		}

		modifieddate = time.Unix(0, 0).UTC()
		for _, fileinfo := range filelist {
			keysize += fileinfo.Size()
			if modifieddate.Before(fileinfo.Modified()) {
				modifieddate = fileinfo.Modified()
			}
		}
	} else { // object path
		isfile = true

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
		keysize = uint64(filesize)

		modifieddate = *resp.LastModified
	}

	return &FileInfo{
		name:     key,
		size:     keysize,
		modified: modifieddate,
		isfile:   isfile,
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
		result[index].isfile = true
	}

	return result, nil
}

// StoreFile writes data from `input` to output URI.
// Output URI must follow the pattern: b2://bucket/path/to/key.
func (b2 *B2Backend) StoreFile(input io.ReadSeeker, uri *url.URL) error {
	var err error
	var bucket string = uri.Host
	var key string = strings.TrimPrefix(uri.Path, "/")
	var contentLength int64
	var bar *progressbar.ProgressBar = nil

	// get content length
	contentLength, err = input.Seek(0, io.SeekEnd)
	_, _ = input.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	// create a progressbar
	if b2.progressEnabled {
		bar = progressbar.NewOptions64(contentLength,
			b2.options...,
		)
	}
	prs := newProgressReadSeeker(input, bar)

	if contentLength > put_get_object_max_bytes {
		// not implemented
		_ = bar.Finish()
		return errors.New(fmt.Sprintf("large file (file size %d > %d bytes), multipart file upload is not implemented", contentLength, put_get_object_max_bytes))
	} else {
		// upload reader contents to S3 bucket as an object with the given key
		_, err = b2.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   prs,
		})
	}

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

// GetProgressEnabled return true if progress output is enabled.
func (b2 *B2Backend) GetProgressEnabled() bool {
	return b2.progressEnabled
}

// SetProgressEnabled enable/disable progress output.
func (b2 *B2Backend) SetProgressEnabled(e bool) {
	b2.progressEnabled = e
}

// GetProgressbarOptions returns progressbar options as a list.
func (b2 *B2Backend) GetProgressbarOptions() []progressbar.Option {
	return b2.options
}

// SetProgressbarOptions sets progressbar options (see progressbar docs).
func (b2 *B2Backend) SetProgressbarOptions(options ...progressbar.Option) {
	b2.options = options
}
