package imagor

import (
	"bytes"
	"io"
	"sync"
)

func fanoutReader(source io.ReadCloser, size int) func(bool) (io.Reader, io.Seeker, io.Closer) {
	var lock sync.RWMutex
	var once sync.Once
	var consumers []chan []byte
	var done = make(chan struct{})
	var closed []bool
	var err error
	var buf []byte
	var curr int

	var init = func() {
		defer func() {
			_ = source.Close()
		}()
		for {
			b := make([]byte, 512)
			n, e := source.Read(b)
			if curr+n > size {
				n = size - curr
			}
			bn := b[:n]

			lock.Lock()
			buf = append(buf, bn...)
			curr += n
			if e != nil {
				if e == io.EOF {
					size = curr
				} else {
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
			if curr >= size {
				close(done)
			}
			lock.RUnlock()
			if e != nil || curr >= size {
				return
			}
		}
	}

	return func(seekable bool) (reader io.Reader, seeker io.Seeker, closer io.Closer) {
		ch := make(chan []byte, size/512+1)

		lock.Lock()
		i := len(consumers)
		consumers = append(consumers, ch)
		closed = append(closed, false)
		cnt := len(buf)
		bufReader := bytes.NewReader(buf)
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

			lock.RLock()
			e = err
			s := size
			c := closed[i]
			ffr := fullBufReader
			rc := readerClosed
			lock.RUnlock()

			if ffr != nil && !rc {
				// proxy to full buf if ready
				return ffr.Read(b)
			}

			if bufReader != nil {
				var e2 error
				n, e2 = bufReader.Read(p)
				if e2 == io.EOF {
					bufReader = nil
					// Don't return EOF yet
				} else if e2 != nil {
					e = e2
				}
				if n > 0 || e != nil {
					return
				}
			}

			if cnt >= s {
				return 0, io.EOF
			}
			if c {
				return 0, io.ErrClosedPipe
			}
			if e != nil {
				_ = closeCh(true)
				return
			}
			if len(b) == 0 {
				b = <-ch
			}
			n = copy(p, b)
			b = b[n:]
			cnt += n
			if cnt >= s {
				_ = closeCh(false)
				e = io.EOF
			}
			return
		})
		if seekable {
			seeker = seekerFunc(func(offset int64, whence int) (int64, error) {
				lock.RLock()
				ffr := fullBufReader
				rc := readerClosed
				lock.RUnlock()
				if ffr != nil && !rc {
					return ffr.Seek(offset, whence)
				} else if ffr == nil && !rc {
					<-done
					lock.Lock()
					fullBufReader = bytes.NewReader(buf)
					ffr = fullBufReader
					if closed[i] {
						lock.Unlock()
					} else {
						closed[i] = true
						lock.Unlock()
						close(ch)
					}
					return ffr.Seek(offset, whence)
				} else {
					return 0, io.ErrClosedPipe
				}
			})
		}
		return
	}
}

type readerFunc func(p []byte) (n int, err error)

func (rf readerFunc) Read(p []byte) (n int, err error) { return rf(p) }

type closerFunc func() error

func (cf closerFunc) Close() error { return cf() }

type seekerFunc func(offset int64, whence int) (int64, error)

func (sf seekerFunc) Seek(offset int64, whence int) (int64, error) { return sf(offset, whence) }
