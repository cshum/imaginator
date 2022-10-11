package fanout

import (
	"bytes"
	"io"
	"sync"
)

type Fanout struct {
	size    int
	lock    sync.RWMutex
	source  io.ReadCloser
	once    sync.Once
	err     error
	readers []*Reader
	buf     []byte
	current int
}

type Reader struct {
	fanout        *Fanout
	channel       chan []byte
	channelClosed bool
	readerClosed  bool
	buf           []byte
	bufReader     *bytes.Reader
	current       int
}

func New(source io.ReadCloser, size int) *Fanout {
	return &Fanout{
		source: source,
		size:   size,
		buf:    make([]byte, size),
	}
}

func (f *Fanout) do() {
	f.once.Do(func() {
		go f.readAll()
	})
}

func (f *Fanout) readAll() {
	defer func() {
		_ = f.source.Close()
	}()
	for {
		b := f.buf[f.current:]
		n, e := f.source.Read(b)
		if f.current+n > f.size {
			n = f.size - f.current
		}
		var bn []byte
		if n > 0 {
			bn = b[:n]
		}
		f.lock.Lock()
		f.current += n
		if e != nil {
			if e == io.EOF {
				e = nil
				if n == 0 {
					if f.current < f.size {
						f.buf = f.buf[:f.current]
					}
					f.size = f.current
				}
			} else {
				f.err = e
			}
		}
		readersCopy := f.readers
		f.lock.Unlock()
		f.lock.RLock()
		for _, r := range readersCopy {
			if !r.channelClosed {
				r.channel <- bn
			}
		}
		f.lock.RUnlock()
		if e != nil || f.current >= f.size {
			return
		}
	}
}

func (f *Fanout) NewReader() *Reader {
	r := &Reader{}
	r.channel = make(chan []byte, f.size/4096+1)
	r.fanout = f

	f.lock.Lock()
	r.current = f.current
	r.bufReader = bytes.NewReader(f.buf[:f.current])
	f.readers = append(f.readers, r)
	f.lock.Unlock()
	return r
}

func (r *Reader) Read(p []byte) (n int, e error) {
	r.fanout.do()
	if r.readerClosed {
		return 0, io.ErrClosedPipe
	}
	if r.bufReader != nil {
		n, e = r.bufReader.Read(p)
		if e == io.EOF {
			r.bufReader = nil
			e = nil
			// Don't return EOF, pass to next reader instead
		} else {
			return
		}
	}
	r.fanout.lock.RLock()
	e = r.fanout.err
	size := r.fanout.size
	closed := r.channelClosed
	r.fanout.lock.RUnlock()

	for {
		if r.current >= size {
			return 0, io.EOF
		}
		if closed {
			return 0, io.ErrClosedPipe
		}
		if e != nil {
			_ = r.close(true)
			return
		}
		if len(r.buf) == 0 {
			r.buf = <-r.channel
		}
		nn := copy(p[n:], r.buf)
		if nn == 0 {
			return
		}
		r.buf = r.buf[nn:]
		r.current += nn
		n += nn
		if r.current >= size {
			_ = r.close(false)
			return
		}
	}
}

func (r *Reader) close(closeReader bool) (e error) {
	r.fanout.lock.Lock()
	e = r.fanout.err
	r.readerClosed = closeReader
	if r.channelClosed {
		r.fanout.lock.Unlock()
	} else {
		r.channelClosed = true
		r.fanout.lock.Unlock()
		close(r.channel)
	}
	return
}

func (r *Reader) Close() error {
	return r.close(true)
}
