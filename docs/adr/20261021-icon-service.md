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

func FromDomain(ctx context.Context, domain string) (icon string, err error)
func FromImageURL(ctx context.Context, rawURL string) (icon string, err error)

// Steps
func faviconURL(domain string) string
func (s *IconFetcher) fetch(ctx context.Context, rawURL string) (body []byte, err error)
func decode(body []byte) (image.Image, error)
func process(img image.Image) ([]byte, error)
func toDataURI(png []byte) string

// Used by fetch
func checkStatus(resp *http.Response) error
func readBody(r io.Reader) ([]byte, error)

// Caller-supplied icons
func Validate(dataURI string) error
func iconPNG(dataURI string) ([]byte, error)
```

- The package-level `FromDomain` and `FromImageURL` call a private default `IconFetcher`, which is never exported.
- `WithHTTPClient` replaces the default `*http.Client`, which is the only dependency tests vary. [fetch](#fetch) applies its body cap whatever client it is given.
- Each step returns one of the package's error sentinels, wrapping its cause with `%w` for logging, so `errors.Is(err, context.DeadlineExceeded)` still works. The sentinels are grouped by the step that fails, not by whether a retry can help: the operations layer maps each one to its own error code (see [operations](./20261018-operations.md#iconfetch-1)).
- Inputs are validated by the operations layer before the service is called: `fromImageUrl` must be an absolute `http`/`https` URL with a host, and `fromDomain` must be non-empty. The service does not repeat these checks.
- Tests use the public API from the external `icon_test` package and never touch the network. Private steps are reached through `FromDomain` and `FromImageURL`, never directly. Behaviour that depends only on the response (status, body, image content) is tested with a fake `RoundTripper` injected through `WithHTTPClient`. Real network behaviour (dropped connections, redirects, deadlines) is tested against an `httptest.Server`.

#### FromDomain

1. [faviconURL](#faviconurl) builds the FaviconDL URL for `domain`.
2. [fetch](#fetch) downloads it.
3. [decode](#decode) decodes the body.
4. [process](#process) brings the image to 64×64 and encodes it as PNG.
5. [toDataURI](#todatauri) wraps the PNG in a data URI, which is returned.

Errors from every step are also wrapped with the domain, so logs show the request went through FaviconDL.

FaviconDL documents that a successful response with `size=64` is a 64×64 PNG, so [normalization](#normalization) usually leaves it unchanged. Decoding and [re-encoding](#re-encoding) stay regardless: the guarantee covers the size and format, not the bytes, so a corrupt body, an error page served with status 200, or extra PNG chunks still have to be rejected or stripped.

#### FromImageURL

1. [fetch](#fetch) downloads `rawURL`.
2. [decode](#decode) decodes the body.
3. [process](#process) brings the image to 64×64 and encodes it as PNG.
4. [toDataURI](#todatauri) wraps the PNG in a data URI, which is returned.

#### faviconURL

Builds `https://favicondl.com/api/extract?url={domain}&size=64` with `url.Values`, never with `fmt.Sprintf`. The base URL is a constant.

##### FaviconDL

- The `url` parameter officially accepts a bare domain (`example.com`) as well as a full URL; FaviconDL normalizes it (lowercases the host, assumes https, strips one leading `www.`).
- The endpoint answers with a 302 to `/api/proxy`, which serves the image.
- FaviconDL [documents](https://favicondl.com/documentation.html) that a request with `size` returns "an exact square PNG" (see [FromDomain](#fromdomain) for what the service still checks).

FaviconDL's responses fit the generic [status table](#status):

| Status | Meaning | Result |
| --- | --- | --- |
| 200 | Icon found (from the site, or from FaviconDL's Google/DuckDuckGo fallback) | the body |
| 400 | `Invalid domain`: the input cannot be read as a hostname | `ErrNotFound` |
| 404 | Neither the site nor the fallback has an icon (`Upstream returned 404`; observed, not documented) | `ErrNotFound` |
| 502, 429, other | `Favicon extraction failed`, rate limiting, outages | `ErrFetchFailed` |

##### Errors

None: building the URL cannot fail.

##### Testing

`FromDomain` runs with an `http.Client` whose `RoundTripper` records each request and returns a fixture response, so no request reaches FaviconDL.

- It requests the exact FaviconDL URL, with the domain escaped (`a b.com`, `x.com?y=1`, a full URL).
- **Contract with FaviconDL:** the 400 and 404 statuses are observed, not documented. A fixture records them.

##### Unnecessary business logic

- **Empty-domain check:** the operations layer already requires `fromDomain`, and an empty domain derived from `iconAuto` gets a 400 from FaviconDL, which is `ErrNotFound` anyway.

#### fetch

Fetches the body at `rawURL` and returns it as received. It knows nothing about images. It composes:

- `checkStatus`: the [status table](#status)
- `readBody`: reads up to the [body cap](#request-limits)

##### Destinations

`rawURL` is validated by the operations layer. Redirects follow the client's defaults: Go's `http.Client` follows at most 10 and refuses any scheme other than `http`/`https`.

Destination addresses are not restricted: loopback and private-network URLs are allowed. Subtill is a single-user app, so whoever can call the service already controls the machine's data. The trade-off is that a caller can make the service request internal URLs (SSRF). The response is only ever returned as a valid 64×64 PNG, never as raw bytes, which limits what can be read that way.

##### Request limits

These limits stop a slow or hostile server from hanging a request or exhausting memory:

- **Timeout:** the caller's `ctx` is the only time limit. The service sets no deadline of its own, and the default client has no `Timeout`. Go's `http.Client` has no overall timeout by default, so a caller that passes a `ctx` without a deadline lets a slow server hang the request; callers (the operations layer) are expected to set one.
- **Body size:** bodies are read through `io.LimitReader` capped at 1 MiB + 1 byte; reading more than 1 MiB means the body is over the cap. Retrying would get the same body, so a larger body is invalid rather than a failed fetch.

The body is read in full before it is decoded, so a read error is a network error, never a truncated image.

##### Status

| Status | Result |
| --- | --- |
| 200 | continue with the body |
| 400, 404, 410 | `ErrNotFound`: the request itself won't succeed on retry |
| other, or network error | `ErrFetchFailed` |

##### Errors

- `ErrNotFound`: status 400, 404 or 410.
- `ErrFetchFailed`: network error, DNS or TLS failure, timeout, cancelled `ctx`, connection dropped while reading the body, too many redirects or a redirect to a non-http scheme, rate limiting (429), and any status not listed in the [status table](#status) (5xx, 401, 403, 204, other 2xx/3xx/4xx). Some causes are permanent (an unknown host, a bad certificate); that is accepted.
- `ErrInvalid`: an empty body, or a body over 1 MiB.

##### Testing

With a fake `RoundTripper`:

- Each status returns its sentinel.
- An empty body and an oversized body return `ErrInvalid`.

Against an `httptest.Server`, which the [destinations](#destinations) policy allows:

- A connection dropped mid-body, more than 10 redirects, a redirect to a non-http scheme, an expired `ctx` deadline and a cancelled `ctx` return `ErrFetchFailed`, with the cause reachable through `errors.Is`.

##### Unnecessary business logic

- **Content-Type filter:** [decode](#decode) detects the format from the body, which is capped at 1 MiB, so checking the header only avoids reading a small body before the same rejection, and risks rejecting valid images served with an odd header.
- **URL policy on the initial URL:** the operations layer validates `fromImageUrl`, and `faviconURL` builds a constant https URL.
- **Redirect check and copied client:** Go's `http.Client` already refuses non-http schemes on redirect and stops after 10 redirects; a custom limit of 5 changes nothing for the caller. The no-credentials rule had no stated threat, since SSRF is accepted.

#### decode

Checks the dimensions with the format's `DecodeConfig`, then fully decodes any supported format.

##### Format detection

The format comes from the body, not from the `Content-Type` header, which can be wrong or generic. `decode` picks the decoder itself from the magic bytes. It does not use the `image` package's global format registry: any package linked into the binary can register a format there, which would silently widen the accepted formats.

Supported formats:

- PNG: `image/png`.
- ICO: a small ICO decoder in the `icon` package. It ignores BMP entries and PNG entries larger than 512×512, measured by the longer side of each entry's own PNG header, not the ICO directory. Of the remaining entries, it picks the smallest one of at least 64×64, otherwise the largest one below 64×64.

Anything else, including JPEG, GIF, WebP, SVG and HTML, is rejected. FaviconDL returns PNG and site favicons are almost always PNG or ICO, so the other raster formats are rare enough to reject. Rasterizing SVG would need a much heavier dependency.

##### Decoded size

Before a full decode, the header is read with the format's `DecodeConfig`; images wider or taller than 512 pixels (`width > 512 || height > 512`) are rejected. This guards against decompression bombs, whose small bodies decode to huge bitmaps, and keeps the downscale to 64×64 cheap. Icons are rarely larger than 512 pixels.

##### Errors

- `ErrInvalid`: an unrecognized format, a corrupt or truncated image, a width or height above 512, or an ICO with no PNG entry of 512×512 or smaller, or only broken ones. Some causes are transient (an HTML error page served with status 200 during an outage); that is accepted.

##### Testing

`FromImageURL` runs with a fake `RoundTripper` serving image fixtures under `testdata/`: PNG and ICO are accepted; an unsupported format (a GIF), a corrupt image, an image wider than 512, an image taller than 512, an ICO whose only PNG entry is larger than 512×512, an ICO with only BMP entries and an ICO without a usable entry return `ErrInvalid`.

##### Unnecessary business logic

- **BMP entries in ICO:** a BMP entry stores a doubled height and an AND mask, which needs its own decoder. FaviconDL already returns PNG, and modern `.ico` files embed PNG, so BMP-only ICOs fetched through `FromImageURL` are rare enough to reject.

#### process

Scales the image into a 64×64 canvas, then encodes it as PNG. This is where every fetched icon, whatever its source format, becomes a 64×64 PNG.

##### Normalization

- **Non-square images** are scaled into a rectangle centred in the 64×64 canvas, with the longer side spanning 64 pixels; the rest stays transparent. No intermediate square canvas is allocated. They are never cropped or stretched.
- **Resizing:** every image is scaled straight into a new 8-bit NRGBA 64×64 canvas with `golang.org/x/image/draw` and Catmull-Rom, for downscaling, upscaling and 64×64 sources alike. Small images are upscaled and may look soft. 16-bit sources come out as 8-bit PNGs.

##### Re-encoding

Every fetched icon is decoded and re-encoded before it is returned; downloaded bytes are never returned as received, even when the body is already a 64×64 PNG. A data URI supplied by the caller is not re-encoded but checked by [Validate](#validate), which accepts the shape re-encoding produces.

- **Extra data is stripped.** `image/png` writes only the chunks needed for the pixels (`IHDR`, `PLTE`, `tRNS`, `IDAT`, `IEND`). Text, metadata, colour profiles and unknown chunks are dropped, so a stored icon is bounded by its pixels: about 16 KiB of raw pixel data for 64×64 8-bit RGBA, and usually a few KiB once compressed.
- **Broken images are rejected.** A full decode reads every `IDAT` chunk and checks its CRC. A truncated or corrupt PNG fails, even when its header is valid and `png.DecodeConfig` would accept it.

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

`FromImageURL` also runs with a fake `RoundTripper` serving fixtures under `testdata/`: a 64×64 fully opaque PNG padded with extra chunks (`tEXt`, `iTXt`, `zTXt`, `eXIf`, `iCCP` and a private ancillary chunk), a truncated PNG (cut in the middle of `IDAT`), and a corrupt PNG (a valid header with a bad `IDAT` CRC and with garbled compressed data).

- **Extra data is stripped:** the PNG with extra chunks is accepted, and the result contains only `IHDR`, `IDAT` and `IEND`, with the same pixels as the input. It is smaller than the input.
- **Broken PNGs are dropped:** the truncated and corrupt PNGs are rejected with `ErrInvalid`.
- **Output is valid:** every icon returned by `FromImageURL` and `FromDomain` in these tests passes `Validate`.

##### Unnecessary business logic

- **Skipping the resize for 64×64 images:** every image has to be drawn into a fresh 8-bit canvas anyway, and Catmull-Rom scaling from 64 to 64 preserves opaque pixels (semi-transparent ones may shift slightly through premultiplied alpha), so a separate branch has no visible effect.

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

`subscription.start` and `subscription.edit` accept an icon as a data URI (see [operations](./20261018-operations.md)). The operations layer checks it with `Validate`, through the `icon` validate tag, and stores it as received. It is rejected unless it is a 64×64 PNG data URI with only the pixel chunks, the shape [process](#process) and [toDataURI](#todatauri) produce, so it keeps the guarantees of [re-encoding](#re-encoding) without being changed. The UI sends icons it got from `icon.fetch`, which always pass.

#### Validate

1. The data URI must be no longer than the prefix plus the base64 length of 36 KiB, so an oversized input is rejected before it is decoded.
2. [iconPNG](#iconpng) extracts the PNG bytes from the data URI.
3. The PNG must be at most 36 KiB, which is enough for any 64×64 PNG with only the pixel chunks from a standard encoder, up to 16-bit RGBA, and keeps the [storage](#consequences) bound.
4. Its chunks must be only `IHDR`, optional `PLTE` and `tRNS`, `IDAT` and `IEND`, and the data must end exactly at `IEND`.
5. `png.DecodeConfig` must report 64×64, before any full decode allocates the image.
6. A full PNG decode, which checks every `IDAT` CRC, must succeed.

It checks the shape, not the bytes: it does not compare the input with what `process` would produce. That output can change between Go versions, and a byte-for-byte check would then reject icons saved before an upgrade.

##### Errors

- `ErrInvalid`: any step above fails. The operations validator for the `icon` field maps it to a validation error on the field, never to `icon_invalid`.

##### Testing

- Accepts the output of `FromImageURL` and `FromDomain` (see [process](#process)).
- Rejects a PNG that isn't 64×64 and a body over 36 KiB with `ErrInvalid`.
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
- **Storage:** a 64×64 PNG data URI is typically a few kilobytes per subscription, and never more than about 48 KiB (the 36 KiB cap of `Validate` in base64), so it is fine to store in the row.
- **Strict caller-supplied icons:** an icon supplied as a data URI is rejected, not cleaned, when it carries metadata or other extra chunks. A client that sends icons from elsewhere must strip them first, or fetch the icon through `icon.fetch`.
- **Fallback icons:** FaviconDL's Google fallback may return a generic globe for sites without an icon. It is accepted as a found icon.
- **Stale icons:** icons are never refreshed automatically; a site's rebrand shows only after the icon is fetched again.
- **Status 400 means "not found" everywhere:** the generic status table maps 400 to `ErrNotFound` for any server, not only FaviconDL, because a rejected request won't succeed on retry.
