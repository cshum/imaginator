package s3storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"io"
)

// S3ReadSeeker implements io.ReadSeekCloser of a given S3 Object.
type S3ReadSeeker struct {
	ctx       context.Context
	s3client  *s3.S3
	bucket    string
	key       string
	head      *s3.HeadObjectOutput
	offset    int64
	size      int64
	lastByte  int64
	chunkSize int
	r         io.ReadCloser
	sink      []byte
}

func NewS3ReadSeeker(
	ctx context.Context,
	s3client *s3.S3,
	bucket string,
	key string,
	chunkSize int,
) *S3ReadSeeker {
	return &S3ReadSeeker{
		ctx:       ctx,
		s3client:  s3client,
		bucket:    bucket,
		key:       key,
		chunkSize: chunkSize,
	}
}

// Seek assumes always can seek to position in S3 object.
// Seeking beyond S3 file size will result failures in Read calls.
func (s *S3ReadSeeker) Seek(offset int64, whence int) (int64, error) {
	discardBytes := 0

	switch whence {
	case io.SeekCurrent:
		discardBytes = int(offset)
		s.offset += offset
	case io.SeekStart:
		// seeking backwards results in dropping current http body.
		// since http body reader can read only forwards.
		if offset < s.offset {
			s.reset()
		}
		discardBytes = int(offset - s.offset)
		s.offset = offset
	case io.SeekEnd:
		if offset > 0 {
			return 0, errors.New("cannot seek beyond end")
		}
		size := s.getSize()
		noffset := int64(size) + offset
		discardBytes = int(noffset - s.offset)
		s.offset = noffset
	default:
		return 0, errors.New("unsupported whence")
	}

	if s.offset > s.lastByte {
		s.reset()
		discardBytes = 0
	}

	if discardBytes > 0 {
		// not seeking
		if discardBytes > len(s.sink) {
			s.sink = make([]byte, discardBytes)
		}
		n, err := s.r.Read(s.sink[:discardBytes])
		if err != nil || n < discardBytes {
			s.reset()
		}
	}

	return s.offset, nil
}

func (s *S3ReadSeeker) Close() error {
	if s.r != nil {
		return s.r.Close()
	}
	return nil
}

func (s *S3ReadSeeker) Read(b []byte) (int, error) {
	if s.r == nil {
		if err := s.fetch(s.chunkSize); err != nil {
			return 0, err
		}
	}

	n, err := s.r.Read(b)
	s.offset += int64(n)

	if err != nil && errors.Is(err, io.EOF) {
		return n, s.fetch(s.chunkSize)
	}

	return n, err
}

func (s *S3ReadSeeker) reset() {
	if s.r != nil {
		s.r.Close()
	}
	s.r = nil
	s.lastByte = 0
}

func (s *S3ReadSeeker) Head() (resp *s3.HeadObjectOutput, err error) {
	if s.head != nil {
		return s.head, nil
	}
	resp, err = s.s3client.HeadObjectWithContext(s.ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key),
	})
	s.head = resp
	return
}

func (s *S3ReadSeeker) getSize() int64 {
	if s.size > 0 {
		return s.size
	}
	resp, err := s.Head()
	if err != nil {
		return 0
	}
	s.size = *resp.ContentLength
	return s.size
}

func (s *S3ReadSeeker) fetch(n int) error {
	s.reset()

	n = min(n, int(s.getSize())-int(s.offset))
	if n <= 0 {
		return io.EOF
	}

	// note, that HTTP Byte Ranges is inclusive range of start-byte and end-byte
	s.lastByte = s.offset + int64(n) - 1
	resp, err := s.s3client.GetObjectWithContext(s.ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", s.offset, s.lastByte)),
	})
	if err != nil {
		return fmt.Errorf("cannot fetch bytes=%d-%d: %w", s.offset, s.lastByte, err)
	}
	s.r = resp.Body
	return nil
}
