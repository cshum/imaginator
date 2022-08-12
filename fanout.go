package imagor

import (
	"bytes"
	"io"
	"sync"
)

// FanoutReader fan-out io.ReadCloser - known size, unknown number of consumers
func FanoutReader(reader io.ReadCloser, size int) func() io.ReadCloser {
	var lock sync.RWMutex
	var once sync.Once
	var consumers []chan []byte
	var closed []bool
	var err error
	var buf []byte
	var total = -1
	var curr int

	var init = func() {
		defer func() {
			_ = reader.Close()
		}()
		for {
			b := make([]byte, 512)
			n, e := reader.Read(b)
			bn := b[:n]

			lock.Lock()
			buf = append(buf, bn...)
			curr += n
			if e != nil {
				total = curr
				if e != io.EOF {
					err = e
				}
			}
			lock.Unlock()
			lock.RLock()
			for i, ch := range consumers {
				if !closed[i] {
					ch <- bn
				}
			}
			lock.RUnlock()
			if e != nil || total > -1 {
				return
			}
		}
	}

	return func() io.ReadCloser {
		ch := make(chan []byte, size/512+1)

		lock.Lock()
		i := len(consumers)
		consumers = append(consumers, ch)
		closed = append(closed, false)
		cnt := len(buf)
		bufReader := bytes.NewReader(buf)
		lock.Unlock()

		closeCh := closerFunc(func() (e error) {
			lock.Lock()
			e = err
			if closed[i] {
				lock.Unlock()
				return
			}
			closed[i] = true
			lock.Unlock()
			close(ch)
			return
		})

		var b []byte

		return &readerCloser{
			Reader: io.MultiReader(
				bufReader,
				readerFunc(func(p []byte) (n int, e error) {
					once.Do(func() {
						go init()
					})

					lock.RLock()
					e = err
					t := total
					c := closed[i]
					lock.RUnlock()

					if t > -1 && cnt >= t && e == nil {
						return 0, io.EOF
					}
					if c {
						return 0, io.ErrClosedPipe
					}
					if e != nil {
						_ = closeCh()
						return
					}
					if len(b) == 0 {
						b = <-ch
					}
					n = copy(p, b)
					b = b[n:]
					cnt += n
					if t > -1 && cnt >= t {
						_ = closeCh()
						e = io.EOF
					}
					return
				}),
			),
			Closer: closeCh,
		}
	}
}

type readerCloser struct {
	io.Reader
	io.Closer
}

type readerFunc func(p []byte) (n int, err error)

func (rf readerFunc) Read(p []byte) (n int, err error) { return rf(p) }

type closerFunc func() error

func (cf closerFunc) Close() error { return cf() }
