package vips

import (
	"context"
	"math"
	"runtime"
	"strings"
	"sync"

	"github.com/cshum/imagor"
	"go.uber.org/zap"
)

// FilterFunc filter handler function
type FilterFunc func(ctx context.Context, img *Image, load imagor.LoadFunc, args ...string) (err error)

// FilterMap filter handler map
type FilterMap map[string]FilterFunc

var processorLock sync.RWMutex
var processorCount int

// Processor implements imagor.Processor interface
type Processor struct {
	Filters            FilterMap
	DisableBlur        bool
	DisableFilters     []string
	MaxFilterOps       int
	Logger             *zap.Logger
	Concurrency        int
	MaxCacheFiles      int
	MaxCacheMem        int
	MaxCacheSize       int
	MaxWidth           int
	MaxHeight          int
	MaxResolution      int
	MaxAnimationFrames int
	MozJPEG            bool
	StripMetadata      bool
	AvifSpeed          int
	Debug              bool

	disableFilters map[string]bool
}

// NewProcessor create Processor
func NewProcessor(options ...Option) *Processor {
	v := &Processor{
		MaxWidth:           9999,
		MaxHeight:          9999,
		MaxResolution:      81000000,
		Concurrency:        1,
		MaxFilterOps:       -1,
		MaxAnimationFrames: -1,
		Logger:             zap.NewNop(),
		disableFilters:     map[string]bool{},
	}
	v.Filters = FilterMap{
		"watermark":        v.watermark,
		"round_corner":     roundCorner,
		"rotate":           rotate,
		"label":            label,
		"grayscale":        grayscale,
		"brightness":       brightness,
		"background_color": backgroundColor,
		"contrast":         contrast,
		"modulate":         modulate,
		"hue":              hue,
		"saturation":       saturation,
		"rgb":              rgb,
		"blur":             blur,
		"sharpen":          sharpen,
		"strip_icc":        stripIcc,
		"strip_exif":       stripExif,
		"trim":             trim,
		"set_frames":       setFrames,
		"padding":          v.padding,
		"proportion":       proportion,
	}
	for _, option := range options {
		option(v)
	}
	if v.DisableBlur {
		v.DisableFilters = append(v.DisableFilters, "blur", "sharpen")
	}
	for _, name := range v.DisableFilters {
		v.disableFilters[name] = true
	}
	if v.Concurrency == -1 {
		v.Concurrency = runtime.NumCPU()
	}
	return v
}

// Startup implements imagor.Processor interface
func (v *Processor) Startup(_ context.Context) error {
	processorLock.Lock()
	defer processorLock.Unlock()
	processorCount++
	if processorCount > 1 {
		return nil
	}
	if v.Debug {
		SetLogging(func(domain string, level LogLevel, msg string) {
			switch level {
			case LogLevelDebug:
				v.Logger.Debug(domain, zap.String("log", msg))
			case LogLevelMessage, LogLevelInfo:
				v.Logger.Info(domain, zap.String("log", msg))
			case LogLevelWarning, LogLevelCritical, LogLevelError:
				v.Logger.Warn(domain, zap.String("log", msg))
			}
		}, LogLevelDebug)
	} else {
		SetLogging(func(domain string, level LogLevel, msg string) {
			v.Logger.Warn(domain, zap.String("log", msg))
		}, LogLevelError)
	}
	Startup(&Config{
		MaxCacheFiles:    v.MaxCacheFiles,
		MaxCacheMem:      v.MaxCacheMem,
		MaxCacheSize:     v.MaxCacheSize,
		ConcurrencyLevel: v.Concurrency,
	})
	return nil
}

// Shutdown implements imagor.Processor interface
func (v *Processor) Shutdown(_ context.Context) error {
	processorLock.Lock()
	defer processorLock.Unlock()
	if processorCount <= 0 {
		return nil
	}
	processorCount--
	if processorCount == 0 {
		Shutdown()
	}
	return nil
}

func newImageFromBlob(
	ctx context.Context, blob *imagor.Blob, params *ImportParams,
) (*Image, error) {
	if blob == nil || blob.IsEmpty() {
		return nil, imagor.ErrNotFound
	}
	if blob.BlobType() == imagor.BlobTypeMemory {
		buf, width, height, bands, _ := blob.Memory()
		return LoadImageFromMemory(buf, width, height, bands)
	}
	reader, _, err := blob.NewReader()
	if err != nil {
		return nil, err
	}
	src := NewSource(reader)
	contextDefer(ctx, src.Close)
	img, err := src.LoadImage(params)
	if err != nil && blob.BlobType() == imagor.BlobTypeBMP {
		// fallback with Go BMP decoder if vips error on BMP
		src.Close()
		r, _, err := blob.NewReader()
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = r.Close()
		}()
		return loadImageFromBMP(r)
	}
	return img, err
}

func newThumbnailFromBlob(
	ctx context.Context, blob *imagor.Blob,
	width, height int, crop Interesting, size Size, params *ImportParams,
) (*Image, error) {
	if blob == nil || blob.IsEmpty() {
		return nil, imagor.ErrNotFound
	}
	reader, _, err := blob.NewReader()
	if err != nil {
		return nil, err
	}
	src := NewSource(reader)
	contextDefer(ctx, src.Close)
	return src.LoadThumbnail(width, height, crop, size, params)
}

// NewThumbnail creates new thumbnail with resize and crop from imagor.Blob
func (v *Processor) NewThumbnail(
	ctx context.Context, blob *imagor.Blob, width, height int, crop Interesting,
	size Size, n, page int, dpi int,
) (*Image, error) {
	var params = NewImportParams()
	if dpi > 0 {
		params.Density.Set(dpi)
	}
	var err error
	var img *Image
	params.FailOnError.Set(false)
	if isMultiPage(blob, n, page) {
		applyMultiPageParams(params, n, page)
		if crop == InterestingNone || size == SizeForce {
			if img, err = newImageFromBlob(ctx, blob, params); err != nil {
				return nil, WrapErr(err)
			}
			if n > 1 || page > 1 {
				// reload image to restrict frames loaded
				n, page = recalculateImage(img, n, page)
				return v.NewThumbnail(ctx, blob, width, height, crop, size, -n, -page, dpi)
			}
			if _, err = v.CheckResolution(img, nil); err != nil {
				return nil, err
			}
			if err = img.ThumbnailWithSize(width, height, crop, size); err != nil {
				img.Close()
				return nil, WrapErr(err)
			}
		} else {
			if img, err = v.CheckResolution(newImageFromBlob(ctx, blob, params)); err != nil {
				return nil, WrapErr(err)
			}
			if n > 1 || page > 1 {
				// reload image to restrict frames loaded
				n, page = recalculateImage(img, n, page)
				return v.NewThumbnail(ctx, blob, width, height, crop, size, -n, -page, dpi)
			}
			if err = v.animatedThumbnailWithCrop(img, width, height, crop, size); err != nil {
				img.Close()
				return nil, WrapErr(err)
			}
		}
	} else {
		switch blob.BlobType() {
		case imagor.BlobTypeJPEG, imagor.BlobTypeGIF, imagor.BlobTypeWEBP:
			// only allow real thumbnail for jpeg gif webp
			img, err = newThumbnailFromBlob(ctx, blob, width, height, crop, size, params)
		default:
			img, err = v.newThumbnailFallback(ctx, blob, width, height, crop, size, params)
		}
	}
	return v.CheckResolution(img, WrapErr(err))
}

func (v *Processor) newThumbnailFallback(
	ctx context.Context, blob *imagor.Blob, width, height int, crop Interesting, size Size, params *ImportParams,
) (img *Image, err error) {
	if img, err = v.CheckResolution(newImageFromBlob(ctx, blob, params)); err != nil {
		return
	}
	if err = img.ThumbnailWithSize(width, height, crop, size); err != nil {
		img.Close()
		return
	}
	return img, WrapErr(err)
}

// NewImage creates new Image from imagor.Blob
func (v *Processor) NewImage(ctx context.Context, blob *imagor.Blob, n, page int, dpi int) (*Image, error) {
	var params = NewImportParams()
	if dpi > 0 {
		params.Density.Set(dpi)
	}
	params.FailOnError.Set(false)
	if isMultiPage(blob, n, page) {
		applyMultiPageParams(params, n, page)
		img, err := v.CheckResolution(newImageFromBlob(ctx, blob, params))
		if err != nil {
			return nil, WrapErr(err)
		}
		// reload image to restrict frames loaded
		if n > 1 || page > 1 {
			n, page = recalculateImage(img, n, page)
			return v.NewImage(ctx, blob, -n, -page, dpi)
		}
		return img, nil
	}
	img, err := v.CheckResolution(newImageFromBlob(ctx, blob, params))
	if err != nil {
		return nil, WrapErr(err)
	}
	return img, nil
}

// Thumbnail handles thumbnail operation
func (v *Processor) Thumbnail(
	img *Image, width, height int, crop Interesting, size Size,
) error {
	if crop == InterestingNone || size == SizeForce || img.Height() == img.PageHeight() {
		return img.ThumbnailWithSize(width, height, crop, size)
	}
	return v.animatedThumbnailWithCrop(img, width, height, crop, size)
}

// FocalThumbnail handles thumbnail with custom focal point
func (v *Processor) FocalThumbnail(img *Image, w, h int, fx, fy float64) (err error) {

	var imageWidth, imageHeight float64
	// exif orientation greater 5-8 are 90 or 270 degrees, w and h swapped
	if img.Orientation() > 4 {
		imageWidth = float64(img.PageHeight())
		imageHeight = float64(img.Width())
	} else {
		imageWidth = float64(img.Width())
		imageHeight = float64(img.PageHeight())
	}

	if float64(w)/float64(h) > float64(imageWidth)/float64(imageHeight) {
		if err = img.Thumbnail(w, v.MaxHeight, InterestingNone); err != nil {
			return
		}
	} else {
		if err = img.Thumbnail(v.MaxWidth, h, InterestingNone); err != nil {
			return
		}
	}
	var top, left float64
	left = float64(img.Width())*fx - float64(w)/2
	top = float64(img.PageHeight())*fy - float64(h)/2
	left = math.Max(0, math.Min(left, float64(img.Width()-w)))
	top = math.Max(0, math.Min(top, float64(img.PageHeight()-h)))
	return img.ExtractArea(int(left), int(top), w, h)
}

func (v *Processor) animatedThumbnailWithCrop(
	img *Image, w, h int, crop Interesting, size Size,
) (err error) {
	if size == SizeDown && img.Width() < w && img.PageHeight() < h {
		return
	}
	var top, left int
	if float64(w)/float64(h) > float64(img.Width())/float64(img.PageHeight()) {
		if err = img.ThumbnailWithSize(w, v.MaxHeight, InterestingNone, size); err != nil {
			return
		}
	} else {
		if err = img.ThumbnailWithSize(v.MaxWidth, h, InterestingNone, size); err != nil {
			return
		}
	}
	if crop == InterestingHigh {
		left = img.Width() - w
		top = img.PageHeight() - h
	} else if crop == InterestingCentre || crop == InterestingAttention {
		left = (img.Width() - w) / 2
		top = (img.PageHeight() - h) / 2
	}
	return img.ExtractArea(left, top, w, h)
}

// CheckResolution check image resolution for image bomb prevention
func (v *Processor) CheckResolution(img *Image, err error) (*Image, error) {
	if err != nil || img == nil {
		return img, err
	}
	if img.Width() > v.MaxWidth || img.PageHeight() > v.MaxHeight ||
		(img.Width()*img.PageHeight()) > v.MaxResolution {
		img.Close()
		return nil, imagor.ErrMaxResolutionExceeded
	}
	return img, nil
}

func isMultiPage(blob *imagor.Blob, n, page int) bool {
	return blob != nil && (blob.SupportsAnimation() || blob.BlobType() == imagor.BlobTypePDF) && ((n != 1 && n != 0) || (page != 1 && page != 0))
}

func applyMultiPageParams(params *ImportParams, n, page int) {
	if page < -1 {
		params.Page.Set(-page - 1)
	} else if n < -1 {
		params.NumPages.Set(-n)
	} else {
		params.NumPages.Set(-1)
	}
}

func recalculateImage(img *Image, n, page int) (int, int) {
	// reload image to restrict frames loaded
	numPages := img.Pages()
	img.Close()
	if page > 1 && page > numPages {
		page = numPages
	} else if n > 1 && n > numPages {
		n = numPages
	}
	return n, page
}

// WrapErr wraps error to become imagor.Error
func WrapErr(err error) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(imagor.Error); ok {
		return e
	}
	msg := strings.TrimSpace(err.Error())
	if strings.HasPrefix(msg, "VipsForeignLoad:") &&
		strings.HasSuffix(msg, "is not in a known format") {
		return imagor.ErrUnsupportedFormat
	}
	return imagor.NewError(msg, 406)
}
