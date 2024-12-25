package s3storage

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cshum/imagor"
	"github.com/cshum/imagor/imagorpath"
)

// S3Storage AWS S3 Storage implements imagor.Storage interface
type S3Storage struct {
	S3         *s3.S3
	Uploader   *s3manager.Uploader
	Downloader *s3manager.Downloader
	Bucket     string

	BaseDir      string
	PathPrefix   string
	ACL          string
	SafeChars    string
	StorageClass string
	Expiration   time.Duration

	safeChars imagorpath.SafeChars
}

// New creates S3Storage
func New(sess *session.Session, bucket string, options ...Option) *S3Storage {
	baseDir := "/"
	if idx := strings.Index(bucket, "/"); idx > -1 {
		baseDir = bucket[idx:]
		bucket = bucket[:idx]
	}
	s := &S3Storage{
		S3:       s3.New(sess),
		Uploader: s3manager.NewUploader(sess),
		Bucket:   bucket,

		BaseDir:    baseDir,
		PathPrefix: "/",
		ACL:        s3.ObjectCannedACLPublicRead,
	}
	for _, option := range options {
		option(s)
	}
	if s.SafeChars == "--" {
		s.safeChars = imagorpath.NewNoopSafeChars()
	} else {
		s.safeChars = imagorpath.NewSafeChars("!\"()*" + s.SafeChars)
		// https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-keys.html#object-key-guidelines-safe-characters
	}

	return s
}

// Path transforms and validates image key for storage path
func (s *S3Storage) Path(image string) (string, bool) {
	image = "/" + imagorpath.Normalize(image, s.safeChars)
	if !strings.HasPrefix(image, s.PathPrefix) {
		return "", false
	}
	return filepath.Join(s.BaseDir, strings.TrimPrefix(image, s.PathPrefix)), true
}

// Get implements imagor.Storage interface
func (s *S3Storage) Get(r *http.Request, image string) (*imagor.Blob, error) {
	ctx := r.Context()
	image, ok := s.Path(image)
	if !ok {
		return nil, imagor.ErrInvalid
	}
	var blob *imagor.Blob
	var once sync.Once
	blob = imagor.NewBlob(func() (io.ReadCloser, int64, error) {
		r := NewS3ReadSeeker(
			ctx, s.S3, s.Bucket, image,
			1<<10*100, // 100 KB
		)
		out, err := r.Head()
		if e, ok := err.(awserr.Error); ok && (e.Code() == s3.ErrCodeNoSuchKey || e.Code() == "NotFound") {
			return nil, 0, imagor.ErrNotFound
		} else if err != nil {
			return nil, 0, err
		}
		once.Do(func() {
			if out.ContentType != nil {
				blob.SetContentType(*out.ContentType)
			}
			if out.ContentLength != nil && out.ETag != nil && out.LastModified != nil {
				blob.Stat = &imagor.Stat{
					Size:         *out.ContentLength,
					ETag:         *out.ETag,
					ModifiedTime: *out.LastModified,
				}
			}
		})
		if s.Expiration > 0 && out.LastModified != nil {
			if time.Now().Sub(*out.LastModified) > s.Expiration {
				return nil, 0, imagor.ErrExpired
			}
		}
		var size int64
		if out.ContentLength != nil {
			size = *out.ContentLength
		}
		return r, size, nil
	})
	return blob, nil
}

// Put implements imagor.Storage interface
func (s *S3Storage) Put(ctx context.Context, image string, blob *imagor.Blob) error {
	image, ok := s.Path(image)
	if !ok {
		return imagor.ErrInvalid
	}
	reader, _, err := blob.NewReader()
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()
	var metadata map[string]*string
	input := &s3manager.UploadInput{
		ACL:          aws.String(s.ACL),
		Body:         reader,
		Bucket:       aws.String(s.Bucket),
		ContentType:  aws.String(blob.ContentType()),
		Metadata:     metadata,
		Key:          aws.String(image),
		StorageClass: aws.String(s.StorageClass),
	}
	_, err = s.Uploader.UploadWithContext(ctx, input)
	return err
}

// Delete implements imagor.Storage interface
func (s *S3Storage) Delete(ctx context.Context, image string) error {
	image, ok := s.Path(image)
	if !ok {
		return imagor.ErrInvalid
	}
	_, err := s.S3.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(image),
	})
	return err
}

// Stat implements imagor.Storage interface
func (s *S3Storage) Stat(ctx context.Context, image string) (stat *imagor.Stat, err error) {
	image, ok := s.Path(image)
	if !ok {
		return nil, imagor.ErrInvalid
	}
	input := &s3.HeadObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(image),
	}
	head, err := s.S3.HeadObjectWithContext(ctx, input)
	if e, ok := err.(awserr.Error); ok && e.Code() == s3.ErrCodeNoSuchKey {
		return nil, imagor.ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &imagor.Stat{
		Size:         *head.ContentLength,
		ETag:         *head.ETag,
		ModifiedTime: *head.LastModified,
	}, nil
}
