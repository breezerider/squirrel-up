package common

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

// B2Backend is struct that holds active B2 session.
type (
	B2Backend struct {
		s3iface.S3API
		pr ProgressReporter
	}

	progressSectionReader struct {
		sr    *io.SectionReader
		pr    ProgressReporter
		part  string
		index int
		read  int64
	}

	partUploadResult struct {
		completedPart *s3.CompletedPart
		err           error
	}

	byPartNum []*s3.CompletedPart
)

const (
	multipart_upload_part_size     = 100 * 1024 * 1024
	multipart_upload_wait_seconds  = 5
	multipart_upload_max_attempts  = 5
	multipart_upload_max_concurent = 4
)

var (
	checkS3Client func(*s3.S3)

	waitfunc func(time.Duration) = func(seconds time.Duration) {
		time.Sleep(time.Duration(time.Second * seconds))
	}
)

// NewReader return a new Reader with a given progress bar.
func newProgressSectionReader(sr *io.SectionReader, pr ProgressReporter, partNumber int) *progressSectionReader {
	var index int = 0
	var part string = ""

	if pr != nil {
		index, _ = pr.CreateFileTask(sr.Size() * 2)
		if partNumber > 0 {
			part = fmt.Sprintf(" part #%d", partNumber)
		}
	}

	return &progressSectionReader{
		sr:    sr,
		pr:    pr,
		part:  part,
		index: index,
		read:  0,
	}
}

// Read will read the data and add the number of bytes to the progressbar for sgnign and uploading.
func (psr *progressSectionReader) Read(p []byte) (n int, err error) {
	n, err = psr.sr.Read(p)

	if psr.pr != nil {
		if psr.read == 0 {
			_ = psr.pr.DescribeTask(psr.index, "signing"+psr.part)
		}

		psr.read += int64(n)
		_ = psr.pr.AdvanceTask(psr.index, int64(n))

		if psr.read == psr.sr.Size() {
			_ = psr.pr.DescribeTask(psr.index, "uploading"+psr.part)
		}
	}

	return
}

// Seek the input stream by forwarding the call.
func (psr *progressSectionReader) Seek(offset int64, whence int) (int64, error) {
	return psr.sr.Seek(offset, whence)
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
		cfg.Internal.Reporter,
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
		case "RequestTimeout":
			return errors.New(ErrOperationTimeout)
		}
	}
	return fmt.Errorf("unknown B2 error (%s).", err.Error())
}

/* Sort by part number. */
func (s byPartNum) Len() int {
	return len(s)
}

func (s byPartNum) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s byPartNum) Less(i, j int) bool {
	return *s[i].PartNumber < *s[j].PartNumber
}

// uploadPart upload a given part of a multipart upload.
func (b2 *B2Backend) uploadPart(wg *sync.WaitGroup, result chan partUploadResult, semaphone chan bool, partNum int, input io.ReadSeeker, length int64, createOutput *s3.CreateMultipartUploadOutput) {
	defer wg.Done()
	<-semaphone

	var uploadOutput *s3.UploadPartOutput
	var attempt int
	var err error

uploadCycle:
	for attempt = 0; attempt < multipart_upload_max_attempts; attempt++ {
		// seek to the beginning of the stream
		_, _ = input.Seek(0, io.SeekStart)

		uploadOutput, err = b2.UploadPart(&s3.UploadPartInput{
			Body:          input,
			Bucket:        createOutput.Bucket,
			Key:           createOutput.Key,
			PartNumber:    aws.Int64(int64(partNum)),
			UploadId:      createOutput.UploadId,
			ContentLength: aws.Int64(length),
		})

		if err == nil {
			// upload attempt succeeded
			break uploadCycle
		} else {
			// wait before the next attempt
			waitfunc(multipart_upload_wait_seconds)
		}
	}

	result <- partUploadResult{
		&s3.CompletedPart{
			ETag:       uploadOutput.ETag,
			PartNumber: aws.Int64(int64(partNum)),
		},
		err,
	}
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
func (b2 *B2Backend) StoreFile(inputStream io.ReaderAt, contentLength int64, uri *url.URL) error {
	var err error
	var bucket string = uri.Host
	var key string = strings.TrimPrefix(uri.Path, "/")

	if contentLength > multipart_upload_part_size {
		// upload in chunks
		var createOutput *s3.CreateMultipartUploadOutput
		createOutput, err = b2.CreateMultipartUpload(&s3.CreateMultipartUploadInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return handleError(err)
		} else if createOutput == nil || createOutput.UploadId == nil {
			return handleError(errors.New("multipart upload failed: no upload id found in server response"))
		}

		// split input into individual parts for upload
		wg := new(sync.WaitGroup)
		result := make(chan partUploadResult)
		semaphore := make(chan bool, multipart_upload_max_concurent)

		var jobsStarted int
		for jobsStarted = 0; jobsStarted < multipart_upload_max_concurent; jobsStarted++ {
			// put a value into the semaphore
			semaphore <- true
		}

		var partNum int
		var position, length int64
		length = multipart_upload_part_size
		for position = 0; position < contentLength; position += multipart_upload_part_size {
			if (position + length) >= contentLength {
				length = contentLength - position
			}

			// bump wait counter and part number
			wg.Add(1)
			partNum++

			// create a section reader with progress tracking for current part
			psr := newProgressSectionReader(io.NewSectionReader(inputStream, position, length), b2.pr, int(partNum))

			// upload part in a coroutine
			go b2.uploadPart(wg, result, semaphore, partNum, psr, length, createOutput)
		}

		// clean up
		go func() {
			wg.Wait()
			close(result)
			close(semaphore)
		}()

		// collect upload statuses
		var completedParts []*s3.CompletedPart
		for result := range result {
			if result.err != nil {
				if err == nil {
					err = result.err
				}
			} else {
				completedParts = append(completedParts, result.completedPart)
			}
			if jobsStarted < partNum {
				// put a value into the semaphore
				semaphore <- true
			}
			jobsStarted++
		}

		if len(completedParts) < partNum || err != nil {
			// abort multipart upload
			_, _ = b2.AbortMultipartUpload(&s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(key),
				UploadId: createOutput.UploadId,
			})
		} else {
			// sort completed parts
			sort.Sort(byPartNum(completedParts))

			// finalize multipart upload
			//var completeOutput *s3.CompleteMultipartUploadOutput
			_, err = b2.CompleteMultipartUpload(&s3.CompleteMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(key),
				UploadId: createOutput.UploadId,
				MultipartUpload: &s3.CompletedMultipartUpload{
					Parts: completedParts,
				},
			})
		}
	} else {
		// create a section reader with progress tracking for whole file
		psr := newProgressSectionReader(io.NewSectionReader(inputStream, 0, contentLength), b2.pr, 0)

		// upload reader contents to S3 bucket as an object with the given key
		_, err = b2.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   psr,
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
