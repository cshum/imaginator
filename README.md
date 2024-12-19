# imagor

[![Test Status](https://github.com/cshum/imagor/workflows/test/badge.svg)](https://github.com/cshum/imagor/actions/workflows/test.yml)
[![Coverage Status](https://coveralls.io/repos/github/cshum/imagor/badge.svg?branch=master)](https://coveralls.io/github/cshum/imagor?branch=master)
[![Docker Hub](https://img.shields.io/badge/docker-shumc/imagor-blue.svg)](https://hub.docker.com/r/shumc/imagor/)
[![GitHub Container Registry](https://ghcr-badge.egpl.dev/cshum/imagor/latest_tag?trim=major&label=ghcr.io&ignore=master,develop&color=%23007ec6)](https://github.com/cshum/imagor/pkgs/container/imagor)
[![Go Reference](https://pkg.go.dev/badge/github.com/cshum/imagor.svg)](https://pkg.go.dev/github.com/cshum/imagor)

imagor is a fast, secure image processing server and Go library.

imagor uses one of the most efficient image processing library
[libvips](https://www.libvips.org/). It is typically 4-8x [faster](https://github.com/libvips/libvips/wiki/Speed-and-memory-use) than using the quickest ImageMagick and GraphicsMagick settings.
imagor implements libvips [streaming](https://www.libvips.org/2019/11/29/True-streaming-for-libvips.html) that facilitates parallel processing pipelines, achieving high network throughput.

imagor features a ton of image processing use cases, available as a HTTP server with first-class Docker support. It adopts the [thumbor](https://thumbor.readthedocs.io/en/latest/usage.html#image-endpoint) URL syntax representing a high-performance drop-in replacement.

imagor is a Go library built with speed, security and extensibility in mind. Alongside there is [imagorvideo](https://github.com/cshum/imagorvideo) bringing video thumbnail capability through ffmpeg C bindings.

### Quick Start

```bash
docker run -p 8000:8000 shumc/imagor -imagor-unsafe -imagor-auto-webp
```

Original images:

```
https://raw.githubusercontent.com/cshum/imagor/master/testdata/gopher.png
https://raw.githubusercontent.com/cshum/imagor/master/testdata/dancing-banana.gif
https://raw.githubusercontent.com/cshum/imagor/master/testdata/gopher-front.png
```

<img src="https://raw.githubusercontent.com/cshum/imagor/master/testdata/gopher.png" height="100" /> <img src="https://raw.githubusercontent.com/cshum/imagor/master/testdata/dancing-banana.gif" height="100" /> <img src="https://raw.githubusercontent.com/cshum/imagor/master/testdata/gopher-front.png" height="100" />

Try out the following image URLs:

```
http://localhost:8000/unsafe/fit-in/200x200/filters:fill(white)/https://raw.githubusercontent.com/cshum/imagor/master/testdata/gopher.png
http://localhost:8000/unsafe/200x200/smart/filters:fill(white):format(jpeg):quality(80)/https://raw.githubusercontent.com/cshum/imagor/master/testdata/gopher.png
http://localhost:8000/unsafe/fit-in/-180x180/10x10/filters:hue(290):saturation(100):fill(yellow)/raw.githubusercontent.com/cshum/imagor/master/testdata/gopher.png
http://localhost:8000/unsafe/30x40:100x150/filters:fill(cyan)/raw.githubusercontent.com/cshum/imagor/master/testdata/dancing-banana.gif
http://localhost:8000/unsafe/fit-in/200x150/filters:fill(yellow):watermark(raw.githubusercontent.com/cshum/imagor/master/testdata/gopher-front.png,repeat,bottom,0,40,40)/raw.githubusercontent.com/cshum/imagor/master/testdata/dancing-banana.gif
```

<img src="https://raw.githubusercontent.com/cshum/imagor/master/testdata/demo1.jpg" height="100" /> <img src="https://raw.githubusercontent.com/cshum/imagor/master/testdata/demo2.jpg" height="100" /> <img src="https://raw.githubusercontent.com/cshum/imagor/master/testdata/demo4.jpg" height="100" /> <img src="https://raw.githubusercontent.com/cshum/imagor/master/testdata/demo3.gif" height="100" /> <img src="https://raw.githubusercontent.com/cshum/imagor/master/testdata/demo5.gif" height="100" />  

### Image Endpoint

imagor endpoint is a series of URL parts which defines the image operations, followed by the image URI:

```
/HASH|unsafe/trim/AxB:CxD/fit-in/stretch/-Ex-F/GxH:IxJ/HALIGN/VALIGN/smart/filters:NAME(ARGS):NAME(ARGS):.../IMAGE
```

- `HASH` is the URL signature hash, or `unsafe` if unsafe mode is used
- `trim` removes surrounding space in images using top-left pixel color
- `AxB:CxD` means manually crop the image at left-top point `AxB` and right-bottom point `CxD`. Coordinates can also be provided as float values between 0 and 1 (percentage of image dimensions)
- `fit-in` means that the generated image should not be auto-cropped and otherwise just fit in an imaginary box specified by `ExF`
- `stretch` means resize the image to `ExF` without keeping its aspect ratios
- `-Ex-F` means resize the image to be `ExF` of width per height size. The minus signs mean flip horizontally and vertically
- `GxH:IxJ` add left-top padding `GxH` and right-bottom padding `IxJ`
- `HALIGN` is horizontal alignment of crop. Accepts `left`, `right` or `center`, defaults to `center`
- `VALIGN` is vertical alignment of crop. Accepts `top`, `bottom` or `middle`, defaults to `middle`
- `smart` means using smart detection of focal points
- `filters` a pipeline of image filter operations to be applied, see filters section
- `IMAGE` is the image path or URI
  - For image URI that contains `?` character, this will interfere the URL query and should be encoded with [`encodeURIComponent`](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/encodeURIComponent) or equivalent

### Filters

Filters `/filters:NAME(ARGS):NAME(ARGS):.../` is a pipeline of image operations that will be sequentially applied to the image. Examples:

```
/filters:fill(white):format(jpeg)/
/filters:hue(290):saturation(100):fill(yellow):format(jpeg):quality(80)/
/filters:fill(white):watermark(raw.githubusercontent.com/cshum/imagor/master/testdata/gopher-front.png,repeat,bottom,10):format(jpeg)/
```

imagor supports the following filters:

- `background_color(color)` sets the background color of a transparent image
  - `color` the color name or hexadecimal rgb expression without the “#” character
- `blur(sigma)` applies gaussian blur to the image
- `brightness(amount)` increases or decreases the image brightness
  - `amount` -100 to 100, the amount in % to increase or decrease the image brightness
- `contrast(amount)` increases or decreases the image contrast
  - `amount` -100 to 100, the amount in % to increase or decrease the image contrast
- `fill(color)` fill the missing area or transparent image with the specified color:
  - `color` - color name or hexadecimal rgb expression without the “#” character
    - If color is "blur" - missing parts are filled with blurred original image
    - If color is "auto" - the top left image pixel will be chosen as the filling color
    - If color is "none" - the filling would become fully transparent
- `focal(AxB:CxD)` or `focal(X,Y)` adds a focal region or focal point for custom transformations:
  - Coordinated by a region of left-top point `AxB` and right-bottom point `CxD`, or a point `X,Y`.
  - Also accepts float values between 0 and 1 that represents percentage of image dimensions.
- `format(format)` specifies the output format of the image
  - `format` accepts jpeg, png, gif, webp, tiff, avif, jp2
- `grayscale()` changes the image to grayscale
- `hue(angle)` increases or decreases the image hue
  - `angle` the angle in degree to increase or decrease the hue rotation
- `label(text, x, y, size, color[, alpha[, font]])` adds a text label to the image. It can be positioned inside the image with the alignment specified, color and transparency support:
  - `text` text label, also support url encoded text.
  - `x` horizontal position that the text label will be in:
    - Positive number indicate position from the left, negative number from the right.
    - Number followed by a `p` e.g. 20p means calculating the value from the image width as percentage
    - `left`,`right`,`center` align left, right or centered respectively
  - `y` vertical position that the text label will be in:
    - Positive number indicate position from the top, negative number from the bottom.
    - Number followed by a `p` e.g. 20p means calculating the value from the image height as percentage
    - `top`,`bottom`,`center` vertical align top, bottom or centered respectively
  - `size` - text label font size
  - `color` - color name or hexadecimal rgb expression without the “#” character
  - `alpha` - text label transparency, a number between 0 (fully opaque) and 100 (fully transparent).
  - `font` - text label font type
- `max_bytes(amount)` automatically degrades the quality of the image until the image is under the specified `amount` of bytes
- `max_frames(n)` limit maximum number of animation frames `n` to be loaded
- `orient(angle)` rotates the image before resizing and cropping, according to the angle value
  - `angle` accepts 0, 90, 180, 270
- `page(num)` specify page number for PDF, or frame number for animated image, starts from 1
- `dpi(num)` specify the dpi to render at for PDF and SVG
- `proportion(percentage)` scales image to the proportion percentage of the image dimension
- `quality(amount)` changes the overall quality of the image, does nothing for png
  - `amount` 0 to 100, the quality level in %
- `rgb(r,g,b)` amount of color in each of the rgb channels in %. Can range from -100 to 100
- `rotate(angle)` rotates the given image according to the angle value
  - `angle` accepts 0, 90, 180, 270
- `round_corner(rx [, ry [, color]])` adds rounded corners to the image with the specified color as background
  - `rx`, `ry` amount of pixel to use as radius. ry = rx if ry is not provided
  - `color` the color name or hexadecimal rgb expression without the “#” character
- `saturation(amount)` increases or decreases the image saturation
  - `amount` -100 to 100, the amount in % to increase or decrease the image saturation
- `sharpen(sigma)` sharpens the image
- `strip_exif()` removes Exif metadata from the resulting image
- `strip_icc()` removes ICC profile information from the resulting image
- `strip_metadata()` removes all metadata from the resulting image
- `upscale()` upscale the image if `fit-in` is used
- `watermark(image, x, y, alpha [, w_ratio [, h_ratio [, resize_mode]]])` adds a watermark to the image. It can be positioned inside the image with the alpha channel specified and optionally resized based on the image size by specifying the ratio
  - `image` watermark image URI, using the same image loader configured for imagor
  - `x` horizontal position that the watermark will be in:
    - Positive number indicate position from the left, negative number from the right.
    - Number followed by a `p` e.g. 20p means calculating the value from the image width as percentage
    - `left`,`right`,`center` positioned left, right or centered respectively
    - `repeat` the watermark will be repeated horizontally
  - `y` vertical position that the watermark will be in:
    - Positive number indicate position from the top, negative number from the bottom.
    - Number followed by a `p` e.g. 20p means calculating the value from the image height as percentage
    - `top`,`bottom`,`center` positioned top, bottom or centered respectively
    - `repeat` the watermark will be repeated vertically
  - `alpha` watermark image transparency, a number between 0 (fully opaque) and 100 (fully transparent).
  - `w_ratio` percentage of the width of the image the watermark should fit-in
  - `h_ratio` percentage of the height of the image the watermark should fit-in
  - `resize_mode` resize mode of the watermark. If 'force', the watermark will be resized to w_ratio and h_ratio regardless of the watermark's aspect ratio. Otherwise, the watermark will be resized to fit within the w_ratio and h_ratio, respecting its aspect ratio.

#### Utility Filters

These filters do not manipulate images but provide useful utilities to the imagor pipeline:

- `attachment(filename)` returns attachment in the `Content-Disposition` header, and the browser will open a "Save as" dialog with `filename`. When `filename` not specified, imagor will get the filename from the image source
- `expire(timestamp)` adds expiration time to the content. `timestamp` is the unix milliseconds timestamp, e.g. if content is valid for 30s then timestamp would be `Date.now() + 30*1000` in JavaScript.
- `preview()` skips the result storage even if result storage is enabled. Useful for conditional caching
- `raw()` response with a raw unprocessed and unchecked source image. Image still loads from loader and storage but skips the result storage


### Loader, Storage and Result Storage

imagor `Loader`, `Storage` and `Result Storage` are the building blocks for loading and saving images from various sources:

- `Loader` loads image. Enable `Loader` where you wish to load images from, but without modifying it e.g. static directory.
- `Storage` loads and saves image. This allows subsequent requests for the same image loads directly from the storage, instead of HTTP source.
- `Result Storage` loads and saves the processed image. This allows subsequent request of the same parameters loads from the result storage, saving processing resources.

imagor provides built-in adaptors that support HTTP(s), Proxy, File System, AWS S3 and Google Cloud Storage. By default, `HTTP Loader` is used as fallback. You can choose to enable additional adaptors that fit your use cases.

#### File System

Docker Compose example with file system, using mounted volume:

```yaml
version: "3"
services:
  imagor:
    image: shumc/imagor:latest
    volumes:
      - ./:/mnt/data
    environment:
      PORT: 8000
      IMAGOR_UNSAFE: 1 # unsafe URL for testing

      FILE_LOADER_BASE_DIR: /mnt/data # enable file loader by specifying base dir

      FILE_STORAGE_BASE_DIR: /mnt/data # enable file storage by specifying base dir
      FILE_STORAGE_MKDIR_PERMISSION: 0755 # optional
      FILE_STORAGE_WRITE_PERMISSION: 0666 # optional

      FILE_RESULT_STORAGE_BASE_DIR: /mnt/data/result # enable file result storage by specifying base dir
      FILE_RESULT_STORAGE_MKDIR_PERMISSION: 0755 # optional
      FILE_RESULT_STORAGE_WRITE_PERMISSION: 0666 # optional
      
    ports:
      - "8000:8000"
```

#### AWS S3

Docker Compose example with AWS S3. Also works with S3 compatible such as MinIO, DigitalOcean Space.

```yaml
version: "3"
services:
  imagor:
    image: shumc/imagor:latest
    environment:
      PORT: 8000
      IMAGOR_SECRET: mysecret # secret key for URL signature
      AWS_ACCESS_KEY_ID: ...
      AWS_SECRET_ACCESS_KEY: ...
      AWS_REGION: ...

      S3_LOADER_BUCKET: mybucket # enable S3 loader by specifying bucket
      S3_LOADER_BASE_DIR: images # optional

      S3_STORAGE_BUCKET: mybucket # enable S3 storage by specifying bucket
      S3_STORAGE_BASE_DIR: images # optional
      S3_STORAGE_ACL: public-read # optional - see https://docs.aws.amazon.com/AmazonS3/latest/userguide/acl-overview.html#canned-acl

      S3_RESULT_STORAGE_BUCKET: mybucket # enable S3 result storage by specifying bucket
      S3_RESULT_STORAGE_BASE_DIR: images/result # optional
      S3_RESULT_STORAGE_ACL: public-read # optional
    ports:
      - "8000:8000"
```

##### Custom S3 Endpoint

Configure custom S3 endpoint for S3 compatible such as MinIO, DigitalOcean Space:

```yaml
      S3_ENDPOINT: http://minio:9000
      S3_FORCE_PATH_STYLE: 1
```

By default, S3 prepends bucket name as subdomain to the request URL:

```
http://mybucket.minio:9000/image.jpg
```

this may not be desirable for a self-hosted endpoint. You can also switch to [path-style requests](https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html#path-style-access) using `S3_FORCE_PATH_STYLE=1` such that the host remains unchanged:

```
http://minio:9000/mybucket/image.jpg
```

##### Different AWS Credentials for S3 Loader, Storage and Result Storage

Set the following environment variables to override the global AWS Credentials for S3 Loader, Storage and Result Storage:

```dotenv
AWS_LOADER_REGION
AWS_LOADER_ACCESS_KEY_ID
AWS_LOADER_SECRET_ACCESS_KEY
S3_LOADER_ENDPOINT

AWS_STORAGE_REGION
AWS_STORAGE_ACCESS_KEY_ID
AWS_STORAGE_SECRET_ACCESS_KEY
S3_STORAGE_ENDPOINT

AWS_RESULT_STORAGE_REGION
AWS_RESULT_STORAGE_ACCESS_KEY_ID
AWS_RESULT_STORAGE_SECRET_ACCESS_KEY
S3_RESULT_STORAGE_ENDPOINT
```

#### Google Cloud Storage

Docker Compose example with Google Cloud Storage:

```yaml
version: "3"
services:
  imagor:
    image: shumc/imagor:latest
    volumes:
      - ./googlesecret:/etc/secrets/google
    environment:
      PORT: 8000
      IMAGOR_SECRET: mysecret # secret key for URL signature
      GOOGLE_APPLICATION_CREDENTIALS: /etc/secrets/google/appcredentials.json # google cloud secrets file

      GCLOUD_LOADER_BUCKET: mybucket # enable loader by specifying bucket
      GCLOUD_LOADER_BASE_DIR: images # optional

      GCLOUD_STORAGE_BUCKET: mybucket # enable storage by specifying bucket
      GCLOUD_STORAGE_BASE_DIR: images # optional
      GCLOUD_STORAGE_ACL: publicRead # optional - see https://cloud.google.com/storage/docs/json_api/v1/objects/insert

      GCLOUD_RESULT_STORAGE_BUCKET: mybucket # enable result storage by specifying bucket
      GCLOUD_RESULT_STORAGE_BASE_DIR: images/result # optional
      GCLOUD_RESULT_STORAGE_ACL: publicRead # optional
    ports:
      - "8000:8000"
```

#### Storage and Result Storage Path Style

`Storage` and `Result Storage` path style enables additional hashing rules to the storage path when loading and saving images:

`IMAGOR_STORAGE_PATH_STYLE=digest`

* `foobar.jpg` becomes `e6/86/1a810ff186b4f747ef85f7c53946f0e6d8cb`

`IMAGOR_RESULT_STORAGE_PATH_STYLE=digest`

* `fit-in/16x17/foobar.jpg` becomes `61/4c/9ba1725e8cdd8263a4ad437c56b35f33deba`

`IMAGOR_RESULT_STORAGE_PATH_STYLE=suffix`

* `166x169/top/foobar.jpg` becomes `foobar.45d8ebb31bd4ed80c26e.jpg`
* `17x19/smart/example.com/foobar` becomes `example.com/foobar.ddd349e092cda6d9c729`

`IMAGOR_RESULT_STORAGE_PATH_STYLE=size`

* `166x169/top/foobar.jpg` becomes `foobar.45d8ebb31bd4ed80c26e_166x169.jpg`
* `17x19/smart/example.com/foobar` becomes `example.com/foobar.ddd349e092cda6d9c729_17x19`

### Security

#### URL Signature

In production environment, it is highly recommended turning off `IMAGOR_UNSAFE` and setting up URL signature using `IMAGOR_SECRET`, to prevent DDoS attacks that abuse multiple image operations.

The URL signature hash is based on SHA digest, created by taking the URL path (excluding `/unsafe/`) with secret. The hash is then Base64 URL encoded.
An example in Node.js:

```javascript
const crypto = require('crypto');

function sign(path, secret) {
  const hash = crypto.createHmac('sha1', secret)
          .update(path)
          .digest('base64')
          .replace(/\+/g, '-').replace(/\//g, '_')
  return hash + '/' + path
}

console.log(sign('500x500/top/raw.githubusercontent.com/cshum/imagor/master/testdata/gopher.png', 'mysecret'))
// cST4Ko5_FqwT3BDn-Wf4gO3RFSk=/500x500/top/raw.githubusercontent.com/cshum/imagor/master/testdata/gopher.png
```

#### Custom HMAC Signer

imagor uses SHA1 HMAC signer by default, the same one used by [thumbor](https://thumbor.readthedocs.io/en/latest/security.html#hmac-method). However, SHA1 is not considered cryptographically secure. If that is a concern it is possible to configure different signing method and truncate length. imagor supports `sha1`, `sha256`, `sha512` signer type:

```dotenv
IMAGOR_SIGNER_TYPE=sha256
IMAGOR_SIGNER_TRUNCATE=40
```

The Node.js example then becomes:

```javascript
const crypto = require('crypto');

function sign(path, secret) {
  const hash = crypto.createHmac('sha256', secret)
          .update(path)
          .digest('base64')
          .slice(0, 40)
          .replace(/\+/g, '-').replace(/\//g, '_')
  return hash + '/' + path
}

console.log(sign('500x500/top/raw.githubusercontent.com/cshum/imagor/master/testdata/gopher.png', 'mysecret'))
// IGEn3TxngivD0jy4uuiZim2bdUCvhcnVi1Nm0xGy/500x500/top/raw.githubusercontent.com/cshum/imagor/master/testdata/gopher.png
```

#### Image Bombs Prevention

imagor checks the image type and its resolution before the actual processing happens. The processing will be rejected if the image dimensions are too big, which protects from so-called "image bombs". You can set the max allowed image resolution and dimensions using `VIPS_MAX_RESOLUTION`, `VIPS_MAX_WIDTH`, `VIPS_MAX_HEIGHT`:

```dotenv
VIPS_MAX_RESOLUTION=16800000
VIPS_MAX_WIDTH=5000
VIPS_MAX_HEIGHT=5000
```

#### Allowed Sources and Base URL

Whitelist specific hosts to restrict loading images only from the allowed sources using `HTTP_LOADER_ALLOWED_SOURCES` or `HTTP_LOADER_ALLOWED_SOURCE_REGEXP`.

- `HTTP_LOADER_ALLOWED_SOURCES` accepts csv wth glob pattern e.g.:

  ```dotenv
  HTTP_LOADER_ALLOWED_SOURCES=*.foobar.com,my.foobar.com,mybucket.s3.amazonaws.com
  ```

- `HTTP_LOADER_ALLOWED_SOURCE_REGEXP` accepts a regular expression matching on the full URL e.g.:

  ```dotenv
  HTTP_LOADER_ALLOWED_SOURCE_REGEXP='^https://raw\.githubusercontent\.com/cshum/imagor/.*'
  ```

Alternatively, it is possible to set a base URL for loading images strictly from one HTTP source. This also trims down the base URL from image endpoint:

Example URL:
```
http://localhost:8000/unsafe/fit-in/200x150/filters:fill(yellow):watermark(raw.githubusercontent.com/cshum/imagor/master/testdata/gopher-front.png,repeat,bottom,0,40,40)/raw.githubusercontent.com/cshum/imagor/master/testdata/dancing-banana.gif
```

With HTTP Loader Base URL config:
```
HTTP_LOADER_BASE_URL=https://raw.githubusercontent.com/cshum/imagor/master
```

The example URL then becomes:
```
http://localhost:8000/unsafe/fit-in/200x150/filters:fill(yellow):watermark(testdata/gopher-front.png,repeat,bottom,0,40,40)/testdata/dancing-banana.gif
```


### Metadata and Exif

imagor provides metadata endpoint that extracts information such as image format, resolution and Exif metadata.
Under the hood, it tries to retrieve data just enough to extract the header, without reading and processing the whole image in memory.

To use the metadata endpoint, add `/meta` right after the URL signature hash before the image operations. Example:

```
http://localhost:8000/unsafe/meta/fit-in/50x50/raw.githubusercontent.com/cshum/imagor/master/testdata/Canon_40D.jpg
```

```jsonc
{
  "format": "jpeg",
  "content_type": "image/jpeg",
  "width": 50,
  "height": 34,
  "orientation": 1,
  "pages": 1,
  "bands": 3,
  "exif": {
    "ApertureValue": "368640/65536",
    "ColorSpace": 1,
    "ComponentsConfiguration": "Y Cb Cr -",
    "Compression": 6,
    "DateTime": "2008:07:31 10:38:11",
    "ISOSpeedRatings": 100,
    "Make": "Canon",
    "MeteringMode": 5,
    "Model": "Canon EOS 40D",
    //...
  }
}
```

Prepending `/params` to the existing endpoint returns the endpoint attributes in JSON form, useful for previewing the endpoint parameters. Example:
```bash
curl 'http://localhost:8000/params/g5bMqZvxaQK65qFPaP1qlJOTuLM=/fit-in/500x400/0x20/filters:fill(white)/raw.githubusercontent.com/cshum/imagor/master/testdata/gopher.png'
```

### Go Library

imagor is a Go library built with speed, security and extensibility in mind.
It facilitates high-level image processing in a modular architecture made up of a series of Go packages:

- [imagor](https://pkg.go.dev/github.com/cshum/imagor) - the imagor core library
- [imagorpath](https://pkg.go.dev/github.com/cshum/imagor/imagorpath) - parse and generate imagor endpoint
- [vips](https://pkg.go.dev/github.com/cshum/imagor/vips) - libvips C bindings with `imagor.Processor` implementation
- [httploader](https://pkg.go.dev/github.com/cshum/imagor/loader/httploader) - HTTP Loader, an `imagor.Loader` implementation
- [filestorage](https://pkg.go.dev/github.com/cshum/imagor/storage/filestorage) - File Storage, an `imagor.Storage` implementation
- [s3storage](https://pkg.go.dev/github.com/cshum/imagor/storage/s3storage) - AWS S3 Storage, an `imagor.Storage` implementation
- [gcloudstorage](https://pkg.go.dev/github.com/cshum/imagor/storage/gcloudstorage) - Google Cloud Storage, an `imagor.Storage` implementation

Install [libvips](https://www.libvips.org/) and enable CGO:
- `brew install vips` for Mac
- `CGO_CFLAGS_ALLOW=-Xpreprocessor` being set to compile Go

See example below and also [examples](https://github.com/cshum/imagor/tree/master/examples) folder for various ways you can use imagor:

```go
package main

import (
	"context"
	"github.com/cshum/imagor"
	"github.com/cshum/imagor/imagorpath"
	"github.com/cshum/imagor/loader/httploader"
	"github.com/cshum/imagor/vips"
	"io"
	"os"
)

func main() {
	app := imagor.New(
		imagor.WithLoaders(httploader.New()),
		imagor.WithProcessors(vips.NewProcessor()),
	)
	ctx := context.Background()
	if err := app.Startup(ctx); err != nil {
		panic(err)
	}
	defer app.Shutdown(ctx)
	blob, err := app.Serve(ctx, imagorpath.Params{
		Image:  "https://raw.githubusercontent.com/cshum/imagor/master/testdata/gopher.png",
		Width:  500,
		Height: 500,
		Smart:  true,
		Filters: []imagorpath.Filter{
			{"fill", "white"},
			{"format", "jpg"},
		},
	})
	if err != nil {
		panic(err)
	}
	reader, _, err := blob.NewReader()
	if err != nil {
		panic(err)
	}
	defer reader.Close()
	file, err := os.Create("gopher.jpg")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if _, err := io.Copy(file, reader); err != nil {
		panic(err)
	}
}
```

### Configuration

imagor supports command-line arguments and environment variables for the arguments equivalent in capitalized snake case, see available options `imagor -h`.
For instances `-imagor-secret` would become `IMAGOR_SECRET`:

```bash
# both are equivalent

imagor -debug -imagor-secret 1234

DEBUG=1 IMAGOR_SECRET=1234 imagor
```

Configuration can also be specified in a `.env` environment variable file and referenced with the `-config` flag:

```bash
imagor -config path/to/config.env
```

config.env:

```dotenv
PORT=8000
IMAGOR_SECRET=mysecret
DEBUG=1
```

#### Available options

```
imagor -h
Usage of imagor:
  -debug
        Debug mode
  -port int
        Server port (default 8000)
  -version
        imagor version
  -config string
        Retrieve configuration from the given file (default ".env")

  -imagor-secret string
        Secret key for signing imagor URL
  -imagor-unsafe
        Unsafe imagor that does not require URL signature. Prone to URL tampering
  -imagor-auto-webp
        Output WebP format automatically if browser supports
  -imagor-auto-avif
        Output AVIF format automatically if browser supports (experimental)
  -imagor-base-params string
        imagor endpoint base params that applies to all resulting images e.g. filters:watermark(example.jpg)
  -imagor-signer-type string
        imagor URL signature hasher type: sha1, sha256, sha512 (default "sha1")
  -imagor-signer-truncate int
        imagor URL signature truncate at length
  -imagor-result-storage-path-style string
        imagor result storage path style: original, digest, suffix (default "original")
  -imagor-storage-path-style string
        imagor storage path style: original, digest (default "original")
  -imagor-cache-header-ttl duration
        imagor HTTP cache header ttl for successful image response (default 168h0m0s)
  -imagor-cache-header-swr duration
        imagor HTTP Cache-Control header stale-while-revalidate for successful image response (default 24h0m0s)
  -imagor-cache-header-no-cache
        imagor HTTP Cache-Control header no-cache for successful image response
  -imagor-request-timeout duration
        Timeout for performing imagor request (default 30s)
  -imagor-load-timeout duration
        Timeout for imagor Loader request, should be smaller than imagor-request-timeout
  -imagor-save-timeout duration
        Timeout for saving image to imagor Storage
  -imagor-process-timeout duration
        Timeout for image processing
  -imagor-process-concurrency int
        Maximum number of image process to be executed simultaneously. Requests that exceed this limit are put in the queue. Set -1 for no limit (default -1)
  -imagor-process-queue-size int
        Maximum number of image process that can be put in the queue. Requests that exceed this limit are rejected with HTTP status 429
  -imagor-base-path-redirect string
        URL to redirect for imagor / base path e.g. https://www.google.com
  -imagor-modified-time-check
        Check modified time of result image against the source image. This eliminates stale result but require more lookups
  -imagor-disable-params-endpoint
        imagor disable /params endpoint
  -imagor-disable-error-body
        imagor disable response body on error

  -server-address string
        Server address
  -server-cors
        Enable CORS
  -server-strip-query-string
        Enable strip query string redirection
  -server-path-prefix string
        Server path prefix
  -server-access-log
        Enable server access log

  -prometheus-bind string
        Specify address and port to enable Prometheus metrics, e.g. :5000, prom:7000
  -prometheus-path string
        Prometheus metrics path (default "/")
        
  -http-loader-allowed-sources string
        HTTP Loader allowed hosts whitelist to load images from if set. Accept csv wth glob pattern e.g. *.google.com,*.github.com.
  -http-loader-base-url string
        HTTP Loader base URL that prepends onto existing image path. This overrides the default scheme option.
  -http-loader-forward-headers string
        Forward request header to HTTP Loader request by csv e.g. User-Agent,Accept
  -http-loader-override-response-headers string
        Override HTTP Loader response header to image response by csv e.g. Cache-Control,Expires
  -http-loader-forward-client-headers
        Forward browser client request headers to HTTP Loader request
  -http-loader-insecure-skip-verify-transport
        HTTP Loader to use HTTP transport with InsecureSkipVerify true
  -http-loader-max-allowed-size int
        HTTP Loader maximum allowed size in bytes for loading images if set
  -http-loader-proxy-urls string
        HTTP Loader Proxy URLs. Enable HTTP Loader proxy only if this value present. Accept csv of proxy urls e.g. http://user:pass@host:port,http://user:pass@host:port
  -http-loader-allowed-source-regexp string
        HTTP Loader allowed hosts regexp to load images from if set. Combines as OR with allowed host glob pattern sources.
  -http-loader-proxy-allowed-sources string
        HTTP Loader Proxy allowed hosts that enable proxy transport, if proxy URLs are set. Accept csv wth glob pattern e.g. *.google.com,*.github.com.
  -http-loader-default-scheme string
        HTTP Loader default scheme if not specified by image path. Set "nil" to disable default scheme. (default "https")
  -http-loader-accept string
        HTTP Loader set request Accept header and validate response Content-Type header (default "*/*") 
  -http-loader-block-link-local-networks
        HTTP Loader rejects connections to link local network IP addresses.
  -http-loader-block-loopback-networks
        HTTP Loader rejects connections to loopback network IP addresses.
  -http-loader-block-private-networks
        HTTP Loader rejects connections to private network IP addresses.
  -http-loader-block-networks string
        HTTP Loader rejects connections to link local network IP addresses. This options takes a comma separated list of networks in CIDR notation e.g ::1/128,127.0.0.0/8.
  -http-loader-disable
        Disable HTTP Loader

  -file-safe-chars string
        File safe characters to be excluded from image key escape. Set -- for no-op
  -file-loader-base-dir string
        Base directory for File Loader. Enable File Loader only if this value present
  -file-loader-path-prefix string
        Base path prefix for File Loader
  -file-result-storage-base-dir string
        Base directory for File Result Storage. Enable File Result Storage only if this value present
  -file-result-storage-mkdir-permission string
        File Result Storage mkdir permission (default "0755")
  -file-result-storage-path-prefix string
        Base path prefix for File Result Storage
  -file-result-storage-write-permission string
        File Storage write permission (default "0666")
  -file-result-storage-expiration duration
        File Result Storage expiration duration e.g. 24h. Default no expiration
  -file-storage-base-dir string
        Base directory for File Storage. Enable File Storage only if this value present
  -file-storage-path-prefix string
        Base path prefix for File Storage
  -file-storage-mkdir-permission string
        File Storage mkdir permission (default "0755")
  -file-storage-write-permission string
        File Storage write permission (default "0666")
  -file-storage-expiration duration
        File Storage expiration duration e.g. 24h. Default no expiration

  -aws-access-key-id string
        AWS Access Key ID. Required if using S3 Loader or S3 Storage
  -aws-region string
        AWS Region. Required if using S3 Loader or S3 Storage
  -aws-secret-access-key string
        AWS Secret Access Key. Required if using S3 Loader or S3 Storage
  -aws-session-token string
        AWS Session Token. Optional temporary credentials token
  -s3-endpoint string
        Optional S3 Endpoint to override default
  -s3-safe-chars string
        S3 safe characters to be excluded from image key escape. Set -- for no-op
  -s3-force-path-style
        S3 force the request to use path-style addressing s3.amazonaws.com/bucket/key, instead of bucket.s3.amazonaws.com/key
  -s3-loader-bucket string
        S3 Bucket for S3 Loader. Enable S3 Loader only if this value present
  -s3-loader-base-dir string
        Base directory for S3 Loader
  -s3-loader-path-prefix string
        Base path prefix for S3 Loader
  -s3-result-storage-bucket string
        S3 Bucket for S3 Result Storage. Enable S3 Result Storage only if this value present
  -s3-result-storage-base-dir string
        Base directory for S3 Result Storage
  -s3-result-storage-path-prefix string
        Base path prefix for S3 Result Storage
  -s3-result-storage-acl string
        Upload ACL for S3 Result Storage (default "public-read")
  -s3-result-storage-expiration duration
        S3 Result Storage expiration duration e.g. 24h. Default no expiration
  -s3-storage-bucket string
        S3 Bucket for S3 Storage. Enable S3 Storage only if this value present
  -s3-storage-base-dir string
        Base directory for S3 Storage
  -s3-storage-path-prefix string
        Base path prefix for S3 Storage
  -s3-storage-acl string
        Upload ACL for S3 Storage (default "public-read")
  -s3-storage-expiration duration
        S3 Storage expiration duration e.g. 24h. Default no expiration
        
  -aws-loader-access-key-id string
        AWS Access Key ID for S3 Loader to override global config
  -aws-loader-region string
        AWS Region for S3 Loader to override global config
  -aws-loader-secret-access-key string
        AWS Secret Access Key for S3 Loader to override global config
  -aws-loader-session-token string
        AWS Session Token for S3 Loader to override global config
  -s3-loader-endpoint string
        Optional S3 Loader Endpoint to override default
  -aws-storage-access-key-id string
        AWS Access Key ID for S3 Storage to override global config
  -aws-storage-region string
        AWS Region for S3 Storage to override global config
  -aws-storage-secret-access-key string
        AWS Secret Access Key for S3 Storage to override global config
  -aws-storage-session-token string
        AWS Session Token for S3 Storage to override global config
  -s3-storage-endpoint string
        Optional S3 Storage Endpoint to override default
  -aws-result-storage-access-key-id string
        AWS Access Key ID for S3 Result Storage to override global config
  -aws-result-storage-region string
        AWS Region for S3 Result Storage to override global config
  -aws-result-storage-secret-access-key string
        AWS Secret Access Key for S3 Result Storage to override global config
  -aws-result-storage-session-token string
        AWS Session Token for S3 Result Storage to override global config
  -s3-result-storage-endpoint string
        Optional S3 Storage Endpoint to override default

  -gcloud-safe-chars string
        Google Cloud safe characters to be excluded from image key escape. Set -- for no-op
  -gcloud-loader-base-dir string
        Base directory for Google Cloud Loader
  -gcloud-loader-bucket string
        Bucket name for Google Cloud Storage Loader. Enable Google Cloud Loader only if this value present
  -gcloud-loader-path-prefix string
        Base path prefix for Google Cloud Loader
  -gcloud-result-storage-acl string
        Upload ACL for Google Cloud Result Storage
  -gcloud-result-storage-base-dir string
        Base directory for Google Cloud Result Storage
  -gcloud-result-storage-bucket string
        Bucket name for Google Cloud Result Storage. Enable Google Cloud Result Storage only if this value present
  -gcloud-result-storage-expiration duration
        Google Cloud Result Storage expiration duration e.g. 24h. Default no expiration
  -gcloud-result-storage-path-prefix string
        Base path prefix for Google Cloud Result Storage
  -gcloud-storage-acl string
        Upload ACL for Google Cloud Storage
  -gcloud-storage-base-dir string
        Base directory for Google Cloud
  -gcloud-storage-bucket string
        Bucket name for Google Cloud Storage. Enable Google Cloud Storage only if this value present
  -gcloud-storage-expiration duration
        Google Cloud Storage expiration duration e.g. 24h. Default no expiration
  -gcloud-storage-path-prefix string
        Base path prefix for Google Cloud Storage
        
  -vips-max-animation-frames int
        VIPS maximum number of animation frames to be loaded. Set 1 to disable animation, -1 for unlimited
  -vips-disable-blur
        VIPS disable blur operations for vips processor
  -vips-disable-filters string
        VIPS disable filters by csv e.g. blur,watermark,rgb
  -vips-max-filter-ops int
        VIPS maximum number of filter operations allowed. Set -1 for unlimited (default -1)
  -vips-max-width int
        VIPS max image width
  -vips-max-height int
        VIPS max image height
  -vips-max-resolution int
        VIPS max image resolution
  -vips-mozjpeg
        VIPS enable maximum compression with MozJPEG. Requires mozjpeg to be installed
  -vips-avif-speed int
        VIPS avif speed, the lowest is at 0 and the fastest is at 9 (Default 5).
  -vips-strip-metadata
        VIPS strips all metadata from the resulting image
        
  -sentry-dsn
        include sentry dsn to integrate imagor with sentry
```
