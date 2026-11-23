# Icon Service

## Context

- Each subscription can have an icon.
- The icon service resolves an icon from a domain or an image URL. It sits below the operations layer, which validates input and calls it (see [operations](./20261018-operations.md#iconfetch)). It does not use the database: copying another subscription's icon is a plain store read, done by the operations layer.

## Decision

- Domains are resolved through FaviconDL. FaviconDL is keyless and answers with an image, so it needs no API client: the service builds its URL and downloads it like any other image URL.
- The `icon` package owns the whole path from URL to stored icon: it fetches raw bytes, then decodes, normalizes and encodes them.
- Icons are stored in the database as data URIs (see [icon format](#icon-format)). Subtill is a single SQLite file, so icons travel with the data and need no asset directory.
- The service fetches an icon only when called. It does not cache or refresh icons.
- There is no fallback beyond FaviconDL's own.

### Packages

- `internal/core/icon`: the icon service and its steps, one file per step.

### Service

```go
type IconFetcher struct {
    httpClient *http.Client
}

func NewIconFetcher(opts ...Option) *IconFetcher
func WithHTTPClient(c *http.Client) Option

func (s *IconFetcher) FromDomain(ctx context.Context, domain string) (icon string, err error)
func (s *IconFetcher) FromImageURL(ctx context.Context, rawURL string) (icon string, err error)

// Steps
func faviconURL(domain string) (string, error)
func (s *IconFetcher) fetch(ctx context.Context, rawURL string) (body []byte, err error)
func decodeImage(body []byte) (image.Image, error)
func processImage(img image.Image) ([]byte, error)
func toDataURI(png []byte) string

// Used by fetch
func validateURL(rawURL string) error
func checkStatus(resp *http.Response) error
func checkContentType(resp *http.Response) error
func readBody(r io.Reader) ([]byte, error)

// Caller-supplied icons
func Validate(dataURI string) error
func iconPNG(dataURI string) ([]byte, error)
```

- `WithHTTPClient` replaces the default `*http.Client`, which is the only dependency tests vary. [fetch](#fetch) applies its request limits whatever client it is given.
- Each step returns one of the package's error sentinels directly, wrapping its cause with `%w` for logging, so `errors.Is(err, context.DeadlineExceeded)` still works. The sentinels are grouped by the step that fails, not by whether a retry can help: the operations layer maps every one to `icon_fetch_failed` (see [operations](./20261018-operations.md#iconfetch)), so callers get no retry signal.
- Tests use the public API from the external `icon_test` package and never touch the network. Private steps are reached through `FromDomain` and `FromImageURL`, never directly.

#### FromDomain

1. [faviconURL](#faviconurl) builds the FaviconDL URL for `domain`.
2. [fetch](#fetch) downloads it.
3. [decodeImage](#decodeimage) decodes the body.
4. [processImage](#processimage) brings the image to 64×64 and encodes it as PNG.
5. [toDataURI](#todatauri) wraps the PNG in a data URI, which is returned.

Errors from every step are also wrapped with the domain, so logs show the request went through FaviconDL.

FaviconDL documents that a successful response with `size=64` is a 64×64 PNG, so [normalization](#normalization) usually leaves it unchanged. Decoding and [re-encoding](#re-encoding) stay regardless: the guarantee covers the size and format, not the bytes, so a corrupt body, an error page served with status 200, or extra PNG chunks still have to be rejected or stripped.

#### FromImageURL

1. [fetch](#fetch) downloads `rawURL`.
2. [decodeImage](#decodeimage) decodes the body.
3. [processImage](#processimage) brings the image to 64×64 and encodes it as PNG.
4. [toDataURI](#todatauri) wraps the PNG in a data URI, which is returned.

#### faviconURL

Builds `https://favicondl.com/api/extract?url={domain}&size=64` with `url.Values`, never with `fmt.Sprintf`. The base URL is a constant.

##### FaviconDL

- The `url` parameter officially accepts a bare domain (`example.com`) as well as a full URL; FaviconDL normalizes it (lowercases the host, assumes https, strips one leading `www.`).
- The endpoint answers with a 302 to `/api/proxy`, which serves the image. The redirect stays on https, so it passes the [URL policy](#url-policy).
- FaviconDL [documents](https://favicondl.com/documentation.html) that a request with `size` returns "an exact square PNG" (see [FromDomain](#fromdomain) for what the service still checks).

FaviconDL's responses fit the generic [status table](#status):

| Status | Meaning | Result |
| --- | --- | --- |
| 200 | Icon found (from the site, or from FaviconDL's Google/DuckDuckGo fallback) | the body |
| 400 | `Invalid domain`: the input cannot be read as a hostname | `ErrNotFound` |
| 404 | Neither the site nor the fallback has an icon (`Upstream returned 404`; observed, not documented) | `ErrNotFound` |
| 502, 429, other | `Favicon extraction failed`, rate limiting, outages | `ErrFetchFailed` |

##### Errors

- `ErrInvalidURL`: `domain` is empty. Returned without any request.

##### Testing

`FromDomain` runs with an `http.Client` whose `RoundTripper` records each request and returns a fixture response, so no request reaches FaviconDL.

- It requests the exact FaviconDL URL, with the domain escaped (`a b.com`, `x.com?y=1`, a full URL).
- An empty domain returns `ErrInvalidURL` without any request.
- **Contract with FaviconDL:** the 400 and 404 statuses are observed, not documented. A fixture records them.

#### fetch

Fetches the body at `rawURL` and returns it as received. It knows nothing about images. It composes:

- `validateURL`: the [URL policy](#url-policy); used for the initial URL and in `CheckRedirect`
- `checkStatus`: the [status table](#status)
- `checkContentType`: the [content type](#content-type) table
- `readBody`: reads up to the [body cap](#request-limits)

##### URL policy

Only absolute `http` and `https` URLs with a non-empty host and no credentials are accepted, for the initial request and every redirect. Anything else (`file:`, `data:`, relative or unparsable URLs, `http://user:pass@host/`) is rejected.

Destination addresses are not restricted: loopback and private-network URLs are allowed. Subtill is a single-user app, so whoever can call the service already controls the machine's data. The trade-off is that a caller can make the service request internal URLs (SSRF). The response is only ever returned as a valid 64×64 PNG, never as raw bytes, which limits what can be read that way.

##### Request limits

These limits stop a slow or hostile server from hanging a request or exhausting memory:

- **Timeout:** each request runs under a 10s deadline set on its `ctx`, so it holds even with a client injected through `WithHTTPClient`. Callers can also cancel or shorten a request through `ctx`.
- **Redirects:** at most 5. `fetch` enforces this and the [URL policy](#url-policy) on its own copy of the `*http.Client`, with its own `CheckRedirect`, so an injected client cannot turn them off.
- **Body size:** bodies are read through `io.LimitReader` capped at 1 MiB + 1 byte; reading more than 1 MiB means the body is over the cap. Retrying would get the same body, so a larger body is invalid rather than a failed fetch.

The body is read in full before it is decoded, so a read error is a network error, never a truncated image.

##### Status

| Status | Result |
| --- | --- |
| 200 | continue with the body |
| 400, 404, 410 | `ErrNotFound`: the request itself won't succeed on retry |
| other, or network error | `ErrFetchFailed` |

##### Content type

The `Content-Type` header is an early filter, not the format: it rejects responses that are clearly not images (HTML error pages, login walls, JSON errors served with status 200) before the body is read. Parameters are ignored.

| `Content-Type` | Result |
| --- | --- |
| `image/*` | continue with the body |
| `application/ico`, `application/x-icon` | continue with the body |
| `application/octet-stream`, `binary/octet-stream` | continue with the body (S3, GCS and many CDNs serve uploads this way) |
| missing | continue with the body |
| anything else (`text/*`, `application/json`, …) | `ErrInvalid`, without reading the body |

A header that passes does not decide the format: `image/svg+xml` passes and is then rejected by [decodeImage](#decodeimage).

##### Errors

- `ErrInvalidURL`: the URL does not parse, is relative, is not `http`/`https`, has an empty host or carries credentials; or a redirect points to such a URL. Checked before any request where possible. The client wraps `CheckRedirect` errors in `*url.Error`, so `fetch` matches `ErrInvalidURL` with `errors.Is` before falling back to `ErrFetchFailed`.
- `ErrNotFound`: status 400, 404 or 410.
- `ErrFetchFailed`: network error, DNS or TLS failure, timeout, cancelled `ctx`, connection dropped while reading the body, more than 5 redirects, rate limiting (429), and any status not listed in the [status table](#status) (5xx, 401, 403, 204, other 2xx/3xx/4xx). Some causes are permanent (an unknown host, a bad certificate); that is accepted.
- `ErrInvalid`: a rejected `Content-Type`, an empty body, or a body over 1 MiB.

##### Testing

`FromImageURL` runs against an `httptest.Server`, which the [URL policy](#url-policy) allows.

- Each `ErrInvalidURL` input, including a redirect to a non-http scheme, returns `ErrInvalidURL` and sends no request to the rejected URL.
- Each status, too many redirects, a connection dropped mid-body and a cancelled `ctx` return their sentinel, with the cause reachable through `errors.Is`.
- Each accepted and rejected `Content-Type`, an empty body and an oversized body.
- An `http.Client` injected with `WithHTTPClient` without a `CheckRedirect` still gets the redirect limit and the URL policy.

#### decodeImage

Checks the dimensions with `image.DecodeConfig`, then fully decodes any supported format.

##### Format detection

The format comes from the body, not from the `Content-Type` header, which can be wrong or generic. Decoders are registered with the `image` package, and `image.Decode` picks one by magic bytes.

Supported formats:

- PNG, JPEG, GIF: standard library.
- WebP: `golang.org/x/image/webp`.
- ICO: a small ICO decoder in the `icon` package. It picks the largest entry, which may be PNG or BMP. Its `DecodeConfig` reports the dimensions from the chosen entry's own header, not the ICO directory, so the [decoded size](#decoded-size) limit applies to the embedded image.

Anything else, including SVG and HTML, is rejected. Rasterizing SVG would need a much heavier dependency.

##### Decoded size

Before a full decode, the header is read with `image.DecodeConfig`; images above 4096×4096 are rejected. This guards against decompression bombs, whose small bodies decode to huge bitmaps.

##### Errors

- `ErrInvalid`: an unrecognized format, a corrupt or truncated image, dimensions above 4096×4096, or an ICO with no entries or only broken ones. Some causes are transient (an HTML error page served with status 200 during an outage); that is accepted.

##### Testing

`FromImageURL` runs against an `httptest.Server` serving image fixtures under `testdata/`: PNG, JPEG, GIF, WebP and ICO are accepted; SVG, HTML, a corrupt image, an image above 4096×4096, an ICO whose embedded PNG is above 4096×4096 and an ICO without a usable entry return `ErrInvalid`.

#### processImage

Pads the image to a square, resizes it to 64×64, then encodes it as PNG. This is where every fetched icon, whatever its source format, becomes a 64×64 PNG.

##### Normalization

- **Animated GIF:** the first frame is used.
- **Non-square images** are centered on a transparent square canvas, with the longer side as its size. They are never cropped or stretched.
- **Resizing** to 64×64 uses `golang.org/x/image/draw` with Catmull-Rom, for both downscaling and upscaling. Small images are upscaled and may look soft. A square 64×64 image is not resized.
- **Bit depth:** the result is always drawn into a new 8-bit NRGBA 64×64 canvas before encoding, even when no resize is needed, so 16-bit sources come out as 8-bit PNGs.

##### Re-encoding

Every fetched icon is decoded and re-encoded before it is returned; downloaded bytes are never returned as received, even when the body is already a 64×64 PNG. A data URI supplied by the caller is not re-encoded but checked by [Validate](#validate), which only accepts the shape re-encoding produces.

- **Extra data is stripped.** `image/png` writes only the chunks needed for the pixels (`IHDR`, `PLTE`, `tRNS`, `IDAT`, `IEND`). Text, metadata, colour profiles and unknown chunks are dropped, so a stored icon is bounded by its pixels: about 16 KiB of raw pixel data for 64×64 8-bit RGBA, and usually a few KiB once compressed.
- **Broken images are rejected.** A full decode reads every `IDAT` chunk and checks its CRC. A truncated or corrupt PNG fails, even when its header is valid and `image.DecodeConfig` would accept it.

This matters because:

- **Privacy:** text and `eXIf` chunks can carry author, software or location metadata. Stripping them keeps that data out of the database, and out of every export, backup or copy of it.
- **Security:** stored icons are rendered by the frontend's webview and returned to MCP and REST clients. Every stored icon is either produced by our own encoder or checked to contain nothing else, so no unchecked bytes from outside are passed downstream.
- **Consistency:** every stored icon has the same shape, a 64×64 PNG with only the pixel chunks, whatever its source.

Colour profiles (`iCCP`, `gAMA`) are not applied, so an icon that relies on one may shift slightly in colour. That is acceptable at 64×64.

##### Errors

- `ErrProcessingFailed`: the image could not be encoded as PNG. Close to an internal error.

##### Testing

- `FromImageURL` turns a non-square fixture into a 64×64 icon with transparent padding, and never crops it.
- `FromDomain` turns a non-64×64 PNG into a 64×64 icon.
- `FromImageURL` turns a 16-bit 64×64 PNG into an 8-bit icon that passes `Validate`.

`FromImageURL` also runs against an `httptest.Server` serving fixtures under `testdata/`: a 64×64 PNG padded with extra chunks (`tEXt`, `iTXt`, `zTXt`, `eXIf`, `iCCP` and a private ancillary chunk), a truncated PNG (cut in the middle of `IDAT`), and a corrupt PNG (a valid header with a bad `IDAT` CRC and with garbled compressed data).

- **Extra data is stripped:** the PNG with extra chunks is accepted, and the result contains only `IHDR`, optional `PLTE`/`tRNS`, `IDAT` and `IEND`, with the same pixels as the input. It is smaller than the input.
- **Broken PNGs are dropped:** the truncated and corrupt PNGs are rejected with `ErrInvalid`.
- **Output is valid:** every icon returned by `FromImageURL` and `FromDomain` in these tests passes `Validate`.

#### toDataURI

Wraps PNG bytes in the [icon format](#icon-format): the `data:image/png;base64,` prefix, then standard base64. Its reverse, [iconPNG](#iconpng), lives next to it, so the format is defined in one place.

##### Icon format

An icon is a 64×64 PNG, encoded as a data URI: `data:image/png;base64,<...>`. This is the value stored in the subscription's `icon` column and returned by every service function.

64px is twice the 32px display size, so icons stay sharp on HiDPI screens.

##### Errors

None: base64 encoding cannot fail.

##### Testing

- Every icon returned by `FromImageURL` and `FromDomain` starts with `data:image/png;base64,` and its payload decodes to the PNG.

### Caller-supplied icons

`subscription.start` and `subscription.edit` accept an icon as a data URI (see [operations](./20261018-operations.md)). The operations layer checks it with `Validate`, through the `icon` validate tag, and stores it as received. It is rejected unless it already has the shape [processImage](#processimage) and [toDataURI](#todatauri) produce, so it keeps the guarantees of [re-encoding](#re-encoding) without being changed. The UI sends icons it got from `icon.fetch`, which always pass.

#### Validate

1. [iconPNG](#iconpng) extracts the PNG bytes from the data URI.
2. The PNG must be at most 32 KiB, which is enough for any 64×64 PNG with only the pixel chunks and keeps the [storage](#consequences) bound.
3. Its chunks must be only `IHDR`, optional `PLTE` and `tRNS`, `IDAT` and `IEND`, and the data must end exactly at `IEND`.
4. `png.DecodeConfig` must report 64×64, before any full decode allocates the image.
5. A full PNG decode, which checks every `IDAT` CRC, must succeed.

It checks the shape, not the bytes: it does not compare the input with what `processImage` would produce. That output can change between Go versions, and a byte-for-byte check would then reject icons saved before an upgrade.

##### Errors

- `ErrInvalid`: any step above fails. The operations validator for the `icon` field maps it to a validation error on the field, never to `icon_fetch_failed`.

##### Testing

- Accepts the output of `FromImageURL` and `FromDomain` (see [processImage](#processimage)).
- Rejects a PNG that isn't 64×64 and a body over 32 KiB with `ErrInvalid`.
- Rejects the 64×64 PNG fixture with extra chunks, once per chunk type (`tEXt`, `iTXt`, `zTXt`, `eXIf`, `iCCP`, a private ancillary chunk), with `ErrInvalid`.
- Rejects the truncated and corrupt PNG fixtures with `ErrInvalid`.
- Rejects a 64×64 PNG with bytes after `IEND`, and a small PNG whose header claims huge dimensions, with `ErrInvalid`.

#### iconPNG

The reverse of [toDataURI](#todatauri): requires the `data:image/png;base64,` prefix and standard base64, and returns the decoded bytes. It does not check that they are a PNG; [Validate](#validate) does.

##### Errors

- `ErrInvalid`: the prefix is missing or the base64 is invalid.

##### Testing

`Validate` rejects an empty string, a URL, a data URI with another media type (`data:image/jpeg;base64,`), and invalid base64, all with `ErrInvalid`.

## Consequences

- **Privacy:** every `FromDomain` call sends the domain to favicondl.com, and through its fallback possibly to Google or DuckDuckGo. Those services learn which subscriptions the user has. `FromImageURL` contacts no third party beyond the given URL.
- **Availability:** `FromDomain` depends on a free, keyless service with fair-use limits. When it is unavailable, `FromDomain` returns `ErrFetchFailed`.
- **Storage:** a 64×64 PNG data URI is typically a few kilobytes per subscription, and never more than about 43 KiB (the 32 KiB cap of `Validate` in base64), so it is fine to store in the row.
- **Strict caller-supplied icons:** an icon supplied as a data URI is rejected, not cleaned, when it carries metadata or other extra chunks. A client that sends icons from elsewhere must strip them first, or fetch the icon through `icon.fetch`.
- **Fallback icons:** FaviconDL's Google fallback may return a generic globe for sites without an icon. It is accepted as a found icon.
- **Stale icons:** icons are never refreshed automatically; a site's rebrand shows only after the icon is fetched again.
- **Status 400 means "not found" everywhere:** the generic status table maps 400 to `ErrNotFound` for any server, not only FaviconDL, because a rejected request won't succeed on retry.
