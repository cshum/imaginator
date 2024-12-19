package vips

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/cshum/imagor"
	"github.com/cshum/imagor/imagorpath"
	"golang.org/x/image/colornames"
)

func (v *Processor) watermark(ctx context.Context, img *Image, load imagor.LoadFunc, args ...string) (err error) {
	ln := len(args)
	if ln < 1 {
		return
	}
	image := args[0]
	if unescape, e := url.QueryUnescape(args[0]); e == nil {
		image = unescape
	}
	var blob *imagor.Blob
	if blob, err = load(image); err != nil {
		return
	}
	var x, y, w, h int
	var across = 1
	var down = 1
	var overlay *Image
	var n = 1
	resizeMode := SizeBoth
	if isAnimated(img) {
		n = -1
	}
	// resize_mode
	if ln >= 7 && args[6] == "force" {
		resizeMode = SizeForce
	}
	// w_ratio h_ratio
	if ln >= 6 {
		w = img.Width()
		h = img.PageHeight()
		if args[4] != "none" {
			w, _ = strconv.Atoi(args[4])
			w = img.Width() * w / 100
		}
		if args[5] != "none" {
			h, _ = strconv.Atoi(args[5])
			h = img.PageHeight() * h / 100
		}

		if overlay, err = v.NewThumbnail(
			ctx, blob, w, h, InterestingNone, resizeMode, n, 1, 0,
		); err != nil {
			return
		}
	} else {
		if overlay, err = v.NewThumbnail(
			ctx, blob, v.MaxWidth, v.MaxHeight, InterestingNone, SizeDown, n, 1, 0,
		); err != nil {
			return
		}
	}
	var overlayN = overlay.Height() / overlay.PageHeight()
	contextDefer(ctx, overlay.Close)
	if overlay.Bands() < 3 {
		if err = overlay.ToColorSpace(InterpretationSRGB); err != nil {
			return
		}
	}
	if err = overlay.AddAlpha(); err != nil {
		return
	}
	w = overlay.Width()
	h = overlay.PageHeight()
	// alpha
	if ln >= 4 {
		alpha, _ := strconv.ParseFloat(args[3], 64)
		alpha = 1 - alpha/100
		if alpha != 1 {
			if err = overlay.Linear([]float64{1, 1, 1, alpha}, []float64{0, 0, 0, 0}); err != nil {
				return
			}
		}
	}
	// x y
	if ln >= 3 {
		if args[1] == "center" {
			x = (img.Width() - overlay.Width()) / 2
		} else if args[1] == imagorpath.HAlignLeft {
			x = 0
		} else if args[1] == imagorpath.HAlignRight {
			x = img.Width() - overlay.Width()
		} else if args[1] == "repeat" {
			x = 0
			across = img.Width()/overlay.Width() + 1
		} else if strings.HasPrefix(strings.TrimPrefix(args[1], "-"), "0.") {
			pec, _ := strconv.ParseFloat(args[1], 64)
			x = int(pec * float64(img.Width()))
		} else if strings.HasSuffix(args[1], "p") {
			x, _ = strconv.Atoi(strings.TrimSuffix(args[1], "p"))
			x = x * img.Width() / 100
		} else {
			x, _ = strconv.Atoi(args[1])
		}
		if args[2] == "center" {
			y = (img.PageHeight() - overlay.PageHeight()) / 2
		} else if args[2] == imagorpath.VAlignTop {
			y = 0
		} else if args[2] == imagorpath.VAlignBottom {
			y = img.PageHeight() - overlay.PageHeight()
		} else if args[2] == "repeat" {
			y = 0
			down = img.PageHeight()/overlay.PageHeight() + 1
		} else if strings.HasPrefix(strings.TrimPrefix(args[2], "-"), "0.") {
			pec, _ := strconv.ParseFloat(args[2], 64)
			y = int(pec * float64(img.PageHeight()))
		} else if strings.HasSuffix(args[2], "p") {
			y, _ = strconv.Atoi(strings.TrimSuffix(args[2], "p"))
			y = y * img.PageHeight() / 100
		} else {
			y, _ = strconv.Atoi(args[2])
		}
		if x < 0 {
			x += img.Width() - overlay.Width()
		}
		if y < 0 {
			y += img.PageHeight() - overlay.PageHeight()
		}
	}
	if across*down > 1 {
		if err = overlay.Embed(0, 0, across*w, down*h, ExtendRepeat); err != nil {
			return
		}
	}
	if err = overlay.EmbedBackgroundRGBA(
		x, y, img.Width(), img.PageHeight(), &ColorRGBA{},
	); err != nil {
		return
	}
	if n := img.Height() / img.PageHeight(); n > overlayN {
		cnt := n / overlayN
		if n%overlayN > 0 {
			cnt++
		}
		if err = overlay.Replicate(1, cnt); err != nil {
			return
		}
	}
	if err = img.Composite(overlay, BlendModeOver, 0, 0); err != nil {
		return
	}
	return
}

func setFrames(_ context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	ln := len(args)
	if ln == 0 {
		return
	}
	newN, _ := strconv.Atoi(args[0])
	if newN < 1 {
		return
	}
	if n := img.Height() / img.PageHeight(); n != newN {
		height := img.PageHeight()
		if err = img.SetPageHeight(img.Height()); err != nil {
			return
		}
		if err = img.Embed(0, 0, img.Width(), height*newN, ExtendRepeat); err != nil {
			return
		}
		if err = img.SetPageHeight(height); err != nil {
			return
		}
	}
	var delay int
	if ln > 1 {
		delay, _ = strconv.Atoi(args[1])
	}
	if delay == 0 {
		delay = 100
	}
	delays := make([]int, newN)
	for i := 0; i < newN; i++ {
		delays[i] = delay
	}
	if err = img.SetPageDelay(delays); err != nil {
		return
	}
	return
}

func (v *Processor) fill(ctx context.Context, img *Image, w, h int, pLeft, pTop, pRight, pBottom int, colour string) (err error) {
	if isRotate90(ctx) {
		tmpW := w
		w = h
		h = tmpW
		tmpPLeft := pLeft
		pLeft = pTop
		pTop = tmpPLeft
		tmpPRight := pRight
		pRight = pBottom
		pBottom = tmpPRight
	}
	c := getColor(img, colour)
	left := (w-img.Width())/2 + pLeft
	top := (h-img.PageHeight())/2 + pTop
	width := w + pLeft + pRight
	height := h + pTop + pBottom
	if colour != "blur" || (colour == "blur" && v.DisableBlur) || isAnimated(img) {
		// fill color
		isTransparent := colour == "none" || colour == "transparent"
		if img.HasAlpha() && !isTransparent {
			if err = img.Flatten(getColor(img, colour)); err != nil {
				return
			}
		}
		if isTransparent {
			if img.Bands() < 3 {
				if err = img.ToColorSpace(InterpretationSRGB); err != nil {
					return
				}
			}
			if err = img.AddAlpha(); err != nil {
				return
			}
			if err = img.EmbedBackgroundRGBA(left, top, width, height, &ColorRGBA{}); err != nil {
				return
			}
		} else if isBlack(c) {
			if err = img.Embed(left, top, width, height, ExtendBlack); err != nil {
				return
			}
		} else if isWhite(c) {
			if err = img.Embed(left, top, width, height, ExtendWhite); err != nil {
				return
			}
		} else {
			if err = img.EmbedBackground(left, top, width, height, c); err != nil {
				return
			}
		}
	} else {
		// fill blur
		var cp *Image
		if cp, err = img.Copy(); err != nil {
			return
		}
		contextDefer(ctx, cp.Close)
		if err = img.ThumbnailWithSize(
			width, height, InterestingNone, SizeForce,
		); err != nil {
			return
		}
		if err = img.GaussianBlur(50); err != nil {
			return
		}
		if err = img.Composite(
			cp, BlendModeOver, left, top); err != nil {
			return
		}
	}
	return
}

func roundCorner(ctx context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	var rx, ry int
	var c *Color
	if len(args) == 0 {
		return
	}
	if a, e := url.QueryUnescape(args[0]); e == nil {
		args[0] = a
	}
	if len(args) == 3 {
		// rx,ry,color
		c = getColor(img, args[2])
		args = args[:2]
	}
	rx, _ = strconv.Atoi(args[0])
	ry = rx
	if len(args) > 1 {
		ry, _ = strconv.Atoi(args[1])
	}

	var rounded *Image
	var w = img.Width()
	var h = img.PageHeight()
	if rounded, err = LoadImageFromBuffer([]byte(fmt.Sprintf(`
		<svg viewBox="0 0 %d %d">
			<rect rx="%d" ry="%d" 
			 x="0" y="0" width="%d" height="%d" 
			 fill="#fff"/>
		</svg>
	`, w, h, rx, ry, w, h)), nil); err != nil {
		return
	}
	contextDefer(ctx, rounded.Close)
	if n := img.Height() / img.PageHeight(); n > 1 {
		if err = rounded.Replicate(1, n); err != nil {
			return
		}
	}
	if err = img.Composite(rounded, BlendModeDestIn, 0, 0); err != nil {
		return
	}
	if c != nil {
		if err = img.Flatten(c); err != nil {
			return
		}
	}
	return nil
}

func label(_ context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	ln := len(args)
	if ln == 0 {
		return
	}
	if a, e := url.QueryUnescape(args[0]); e == nil {
		args[0] = a
	}
	var text = args[0]
	var font = "tahoma"
	var x, y int
	var c = &Color{}
	var alpha float64
	var align = AlignLow
	var size = 20
	var width = img.Width()
	if ln > 3 {
		size, _ = strconv.Atoi(args[3])
	}
	if ln > 1 {
		if args[1] == "center" {
			align = AlignCenter
			x = width / 2
		} else if args[1] == imagorpath.HAlignRight {
			align = AlignHigh
			x = width
		} else if strings.HasPrefix(strings.TrimPrefix(args[1], "-"), "0.") {
			pec, _ := strconv.ParseFloat(args[1], 64)
			x = int(pec * float64(width))
		} else if strings.HasSuffix(args[1], "p") {
			x, _ = strconv.Atoi(strings.TrimSuffix(args[1], "p"))
			x = x * width / 100
		} else {
			x, _ = strconv.Atoi(args[1])
		}
		if x < 0 {
			align = AlignHigh
			x += width
		}
	}
	if ln > 2 {
		if args[2] == "center" {
			y = (img.PageHeight() - size) / 2
		} else if args[2] == imagorpath.VAlignTop {
			y = 0
		} else if args[2] == imagorpath.VAlignBottom {
			y = img.PageHeight() - size
		} else if strings.HasPrefix(strings.TrimPrefix(args[2], "-"), "0.") {
			pec, _ := strconv.ParseFloat(args[2], 64)
			y = int(pec * float64(img.PageHeight()))
		} else if strings.HasSuffix(args[2], "p") {
			y, _ = strconv.Atoi(strings.TrimSuffix(args[2], "p"))
			y = y * img.PageHeight() / 100
		} else {
			y, _ = strconv.Atoi(args[2])
		}
		if y < 0 {
			y += img.PageHeight() - size
		}
	}
	if ln > 4 {
		c = getColor(img, args[4])
	}
	if ln > 5 {
		alpha, _ = strconv.ParseFloat(args[5], 64)
		alpha /= 100
	}
	if ln > 6 {
		if a, e := url.QueryUnescape(args[6]); e == nil {
			font = a
		} else {
			font = args[6]
		}
	}
	if img.Bands() < 3 {
		if err = img.ToColorSpace(InterpretationSRGB); err != nil {
			return
		}
	}
	if err = img.AddAlpha(); err != nil {
		return
	}
	return img.Label(text, font, x, y, size, align, c, 1-alpha)
}

func (v *Processor) padding(ctx context.Context, img *Image, _ imagor.LoadFunc, args ...string) error {
	ln := len(args)
	if ln < 2 {
		return nil
	}
	var (
		c       = args[0]
		left, _ = strconv.Atoi(args[1])
		top     = left
		right   = left
		bottom  = left
	)
	if ln > 2 {
		top, _ = strconv.Atoi(args[2])
		bottom = top
	}
	if ln > 4 {
		right, _ = strconv.Atoi(args[3])
		bottom, _ = strconv.Atoi(args[4])
	}
	return v.fill(ctx, img, img.Width(), img.PageHeight(), left, top, right, bottom, c)
}

func backgroundColor(_ context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	if len(args) == 0 {
		return
	}
	if !img.HasAlpha() {
		return
	}
	return img.Flatten(getColor(img, args[0]))
}

func rotate(ctx context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	if len(args) == 0 {
		return
	}
	if angle, _ := strconv.Atoi(args[0]); angle > 0 {
		switch angle {
		case 90, 270:
			setRotate90(ctx)
		}
		if err = img.Rotate(getAngle(angle)); err != nil {
			return err
		}
	}
	return
}

func getAngle(angle int) Angle {
	switch angle {
	case 90:
		return Angle270
	case 180:
		return Angle180
	case 270:
		return Angle90
	default:
		return Angle0
	}
}

func proportion(_ context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	if len(args) == 0 {
		return
	}
	scale, _ := strconv.ParseFloat(args[0], 64)
	if scale <= 0 {
		return // no ops
	}
	if scale > 100 {
		scale = 100
	}
	if scale > 1 {
		scale /= 100
	}
	width := int(float64(img.Width()) * scale)
	height := int(float64(img.PageHeight()) * scale)
	if width <= 0 || height <= 0 {
		return // op ops
	}
	return img.Thumbnail(width, height, InterestingNone)
}

func grayscale(_ context.Context, img *Image, _ imagor.LoadFunc, _ ...string) (err error) {
	return img.ToColorSpace(InterpretationBW)
}

func brightness(_ context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	if len(args) == 0 {
		return
	}
	b, _ := strconv.ParseFloat(args[0], 64)
	b = b * 255 / 100
	return linearRGB(img, []float64{1, 1, 1}, []float64{b, b, b})
}

func contrast(_ context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	if len(args) == 0 {
		return
	}
	a, _ := strconv.ParseFloat(args[0], 64)
	a = a * 255 / 100
	a = math.Min(math.Max(a, -255), 255)
	a = (259 * (a + 255)) / (255 * (259 - a))
	b := 128 - a*128
	return linearRGB(img, []float64{a, a, a}, []float64{b, b, b})
}

func hue(_ context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	if len(args) == 0 {
		return
	}
	h, _ := strconv.ParseFloat(args[0], 64)
	return img.Modulate(1, 1, h)
}

func saturation(_ context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	if len(args) == 0 {
		return
	}
	s, _ := strconv.ParseFloat(args[0], 64)
	s = 1 + s/100
	return img.Modulate(1, s, 0)
}

func rgb(_ context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	if len(args) != 3 {
		return
	}
	r, _ := strconv.ParseFloat(args[0], 64)
	g, _ := strconv.ParseFloat(args[1], 64)
	b, _ := strconv.ParseFloat(args[2], 64)
	r = r * 255 / 100
	g = g * 255 / 100
	b = b * 255 / 100
	return linearRGB(img, []float64{1, 1, 1}, []float64{r, g, b})
}

func modulate(_ context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	if len(args) != 3 {
		return
	}
	b, _ := strconv.ParseFloat(args[0], 64)
	s, _ := strconv.ParseFloat(args[1], 64)
	h, _ := strconv.ParseFloat(args[2], 64)
	b = 1 + b/100
	s = 1 + s/100
	return img.Modulate(b, s, h)
}

func blur(ctx context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	if isAnimated(img) {
		// skip animation support
		return
	}
	var sigma float64
	switch len(args) {
	case 2:
		sigma, _ = strconv.ParseFloat(args[1], 64)
		break
	case 1:
		sigma, _ = strconv.ParseFloat(args[0], 64)
		break
	}
	sigma /= 2
	if sigma > 0 {
		return img.GaussianBlur(sigma)
	}
	return
}

func sharpen(ctx context.Context, img *Image, _ imagor.LoadFunc, args ...string) (err error) {
	if isAnimated(img) {
		// skip animation support
		return
	}
	var sigma float64
	switch len(args) {
	case 1:
		sigma, _ = strconv.ParseFloat(args[0], 64)
		break
	case 2, 3:
		sigma, _ = strconv.ParseFloat(args[1], 64)
		break
	}
	sigma = 1 + sigma*2
	if sigma > 0 {
		return img.Sharpen(sigma, 1, 2)
	}
	return
}

func stripIcc(_ context.Context, img *Image, _ imagor.LoadFunc, _ ...string) (err error) {
	return img.RemoveICCProfile()
}

func stripExif(_ context.Context, img *Image, _ imagor.LoadFunc, _ ...string) (err error) {
	return img.RemoveExif()
}

func trim(ctx context.Context, img *Image, _ imagor.LoadFunc, args ...string) error {
	var (
		ln        = len(args)
		pos       string
		tolerance int
	)
	if ln > 0 {
		tolerance, _ = strconv.Atoi(args[0])
	}
	if ln > 1 {
		pos = args[1]
	}
	if l, t, w, h, err := findTrim(ctx, img, pos, tolerance); err == nil {
		return img.ExtractArea(l, t, w, h)
	}
	return nil
}

func linearRGB(img *Image, a, b []float64) error {
	if img.HasAlpha() {
		a = append(a, 1)
		b = append(b, 0)
	}
	return img.Linear(a, b)
}

func isBlack(c *Color) bool {
	return c.R == 0x00 && c.G == 0x00 && c.B == 0x00
}

func isWhite(c *Color) bool {
	return c.R == 0xff && c.G == 0xff && c.B == 0xff
}

func getColor(img *Image, color string) *Color {
	vc := &Color{}
	args := strings.Split(strings.ToLower(color), ",")
	mode := ""
	name := strings.TrimPrefix(args[0], "#")
	if len(args) > 1 {
		mode = args[1]
	}
	if name == "auto" {
		if img != nil {
			x := 0
			y := 0
			if mode == "bottom-right" {
				x = img.Width() - 1
				y = img.PageHeight() - 1
			}
			p, _ := img.GetPoint(x, y)
			if len(p) >= 3 {
				vc.R = uint8(p[0])
				vc.G = uint8(p[1])
				vc.B = uint8(p[2])
			}
		}
	} else if c, ok := colornames.Map[name]; ok {
		vc.R = c.R
		vc.G = c.G
		vc.B = c.B
	} else if c, ok := parseHexColor(name); ok {
		vc.R = c.R
		vc.G = c.G
		vc.B = c.B
	}
	return vc
}

func parseHexColor(s string) (c color.RGBA, ok bool) {
	c.A = 0xff
	switch len(s) {
	case 6:
		c.R = hexToByte(s[0])<<4 + hexToByte(s[1])
		c.G = hexToByte(s[2])<<4 + hexToByte(s[3])
		c.B = hexToByte(s[4])<<4 + hexToByte(s[5])
		ok = true
	case 3:
		c.R = hexToByte(s[0]) * 17
		c.G = hexToByte(s[1]) * 17
		c.B = hexToByte(s[2]) * 17
		ok = true
	}
	return
}

func hexToByte(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	}
	return 0
}

func isAnimated(img *Image) bool {
	return img.Height() > img.PageHeight()
}
