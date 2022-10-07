package imagor

import (
	"bytes"
	"io"
	"sync"
)

func fanoutReader(source io.ReadCloser, size int) func() (io.Reader, io.Seeker, io.Closer) {
	var lock sync.RWMutex
	var once sync.Once
	var consumers []chan []byte
	var done = make(chan struct{})
	var closed []bool
	var err error
	var buf = make([]byte, size)
	var currentSize int

	var init = func() {
		defer func() {
			_ = source.Close()
		}()
		for {
			n, e := source.Read(buf[currentSize:])
			var bn []byte
			if n > 0 {
				bn = buf[currentSize:n]
			}
			lock.Lock()
			currentSize += n
			if e != nil {
				if e == io.EOF {
					e = nil
					if n == 0 {
						if currentSize < size {
							buf = buf[:currentSize]
						}
						size = currentSize
					}
				} else {
					err = e
				}
			}
			consumersCopy := consumers
			lock.Unlock()
			lock.RLock()
			for i, ch := range consumersCopy {
				if !closed[i] {
					ch <- bn
				}
			}
			lock.RUnlock()
			if currentSize >= size {
				close(done)
			}
			if e != nil || currentSize >= size {
				return
			}
		}
	}

	return func() (reader io.Reader, seeker io.Seeker, closer io.Closer) {
		ch := make(chan []byte, size/4096+1)

		lock.Lock()
		i := len(consumers)
		consumers = append(consumers, ch)
		closed = append(closed, false)
		cnt := currentSize
		bufReader := bytes.NewReader(buf[:currentSize])
		lock.Unlock()

		var readerClosed bool
		var fullBufReader *bytes.Reader

		var b []byte
		var closeCh = func(closeReader bool) (e error) {
			lock.Lock()
			e = err
			readerClosed = closeReader
			if closed[i] {
				lock.Unlock()
			} else {
				closed[i] = true
				lock.Unlock()
				close(ch)
			}
			return
		}
		closer = closerFunc(func() error {
			return closeCh(true)
		})
		reader = readerFunc(func(p []byte) (n int, e error) {
			once.Do(func() {
				go init()
			})
			if readerClosed {
				return 0, io.ErrClosedPipe
			}
			if fullBufReader != nil && !readerClosed {
				// proxy to full buf if ready
				return fullBufReader.Read(p)
			}
			if bufReader != nil {
				n, e = bufReader.Read(p)
				if e == io.EOF {
					bufReader = nil
					e = nil
					// Don't return EOF, pass to next reader instead
				} else if n > 0 || e != nil {
					return
				}
			}

			lock.RLock()
			e = err
			sizeCopy := size
			closedCopy := closed[i]
			lock.RUnlock()

			for {
				if cnt >= sizeCopy {
					return 0, io.EOF
				}
				if closedCopy {
					return 0, io.ErrClosedPipe
				}
				if e != nil {
					_ = closeCh(true)
					return
				}
				if len(b) == 0 {
					b = <-ch
				}
				nn := copy(p[n:], b)
				if nn == 0 {
					return
				}
				b = b[nn:]
				cnt += nn
				n += nn
				if cnt >= sizeCopy {
					_ = closeCh(false)
					return
				}
			}
		})
		seeker = seekerFunc(func(offset int64, whence int) (int64, error) {
			once.Do(func() {
				go init()
			})

			if fullBufReader != nil && !readerClosed {
				return fullBufReader.Seek(offset, whence)
			} else if fullBufReader == nil && !readerClosed {
				<-done
				fullBufReader = bytes.NewReader(buf)
				_ = closeCh(false)
				return fullBufReader.Seek(offset, whence)
			} else {
				return 0, io.ErrClosedPipe
			}
		})
		return
	}
}

type readerFunc func(p []byte) (n int, err error)

func (rf readerFunc) Read(p []byte) (n int, err error) { return rf(p) }

type closerFunc func() error

func (cf closerFunc) Close() error { return cf() }

type seekerFunc func(offset int64, whence int) (int64, error)

func (sf seekerFunc) Seek(offset int64, whence int) (int64, error) { return sf(offset, whence) }
