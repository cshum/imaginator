package vips

import "C"
import (
	"errors"
	"github.com/cshum/imagor/vips/pointer"
	"io"
	"reflect"
	"unsafe"
)

//export goLoggingHandler
func goLoggingHandler(domain *C.char, level C.int, message *C.char) {
	log(C.GoString(domain), LogLevel(level), C.GoString(message))
}

//export goSourceRead
func goSourceRead(
	ptr unsafe.Pointer, buffer unsafe.Pointer, size C.longlong,
) C.longlong {
	src, ok := pointer.Restore(ptr).(*Source)
	if !ok {
		return -1
	}
	// https://stackoverflow.com/questions/51187973/how-to-create-an-array-or-a-slice-from-an-array-unsafe-pointer-in-golang
	sh := &reflect.SliceHeader{
		Data: uintptr(buffer),
		Len:  int(size),
		Cap:  int(size),
	}
	buf := *(*[]byte)(unsafe.Pointer(sh))

	n, err := src.reader.Read(buf)
	if errors.Is(err, io.EOF) {
		return C.longlong(n)
	} else if err != nil {
		return -1
	}
	return C.longlong(n)
}

//export goSourceSeek
func goSourceSeek(
	ptr unsafe.Pointer, offset C.longlong, whence int,
) C.longlong {
	src, ok := pointer.Restore(ptr).(*Source)
	if !ok {
		return -1
	}
	if src.seeker == nil {
		return -1
	}
	switch whence {
	case io.SeekStart, io.SeekCurrent, io.SeekEnd:
	default:
		return -1
	}
	n, err := src.seeker.Seek(int64(offset), whence)
	if err != nil {
		return -1
	}
	return C.longlong(n)
}
