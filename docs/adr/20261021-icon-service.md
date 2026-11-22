# Icon Service

## Context

- Each subscription can have an icon, shown next to its name in every adapter (see [UI PRD](../prd/20261103-subtill-ui.md)).
- The `icon.fetch` operation resolves an icon from a domain, an image URL, or another subscription (see [operations](./20261018-operations.md#iconfetch)). `iconAuto` on `subscription.start` / `subscription.edit` resolves it from the subscription name, used as a domain.
- Subscription names are free text, but by convention start with the service's domain, optionally followed by a path: `netflix.com`, `patreon.com/foo-creator`. Only names that follow this convention can have their icon fetched automatically.
- There is no standard favicon API. The options were compared in the [external APIs review](../prd/20261022-external-apis-review.md):
  - Google s2 and DuckDuckGo are undocumented and unsupported, and DuckDuckGo returns ICO files of varying sizes.
  - Fetching the site and parsing `<link rel=icon>`, the manifest and `/favicon.ico` ourselves (as self-hosted besticon does) keeps domains private, but means writing and maintaining an HTML scraper and handling SVG.
  - FaviconDL is documented, needs no key, does that discovery for us, falls back to Google/DuckDuckGo itself, and returns an exact square PNG at a requested size.

## Decision

- Domains are resolved through FaviconDL. Arbitrary image URLs are downloaded and normalized by our own image client.
- Icons are stored in the database as data URIs (see [icon format](#icon-format)). Subtill is a single SQLite file, so icons travel with the data, need no asset directory, and every adapter can return them as-is.
- An icon is fetched only when a request asks for it (`icon.fetch`, `iconAuto`). The service does not cache or refresh icons: a stored icon is replaced only when the subscription is edited.
- There is no fallback beyond FaviconDL's own. In particular, the subscription's `url` is not used to derive a domain.

### Packages

Following the [currency service](./20261113-currency-service.md) layout:

- `internal/core/clients/favicondl`: the FaviconDL client.
- `internal/core/clients/image`: the image client.
- `internal/core/icon`: the icon service. It is called by the operations behind `icon.fetch` and `iconAuto`, so it serves every adapter (CLI, REST, MCP, desktop) through them.

### Service

```go
type IconService struct {
    domains DomainIconFetcher
    images  ImageIconFetcher
}

func NewIconService(domains DomainIconFetcher, images ImageIconFetcher) *IconService

func (s *IconService) FromDomain(ctx context.Context, domain string) (icon string, err error)
func (s *IconService) FromImageURL(ctx context.Context, rawURL string) (icon string, err error)
func SubscriptionIcon(q sqlx.Ext, subscriptionName string) (icon string, err error)
```

- `FromDomain` and `FromImageURL` fetch over the network and map to `icon.fetch`'s `fromDomain` and `fromImageUrl`.
- `SubscriptionIcon` maps to `fromSubscription`. It does no network fetch: it reads the stored icon of an existing subscription so it can be reused for another one. The lookup key is the subscription **name** (case-insensitive, through `store.Subscriptions.GetByName`), not the ID: CLI and MCP users know their subscriptions by name and never see IDs. It is a package function, not a method, because it is the only one that needs the database. It takes `q sqlx.Ext` like every `store` method, so the caller decides between the pool and a transaction.
- `DomainIconFetcher` and `ImageIconFetcher` are defined by the service and list only the method it calls. The two clients satisfy them.

### Usage

**Domain from a subscription name.** The domain is the part of the name before the first `/`: `patreon.com/foo-creator` gives `patreon.com`. The caller extracts it before calling `FromDomain`. The full name is unchanged; in the UI, the same prefix also becomes the default group (see [UI PRD](../prd/20261103-subtill-ui.md)). That is the UI for autofetch, and the operations for `iconAuto`. A name without a domain-like prefix (e.g. `Netflix`) makes FaviconDL answer `400 Invalid domain`, hence `ErrNotFound`.

**Desktop UI: Add Subscription drawer** (see [UI PRD](../prd/20261103-subtill-ui.md)):

- **Autofetch:** when the user fills in the name, the drawer calls `icon.fetch` with `fromDomain` set to the name's domain, and shows the result on the icon button. It only calls when the domain changes and looks like a domain (it contains a dot), and waits until the user pauses typing, so FaviconDL gets one request per domain rather than one per keystroke. A failure is silent: the icon stays empty and the user can set it manually.
- **Manual:** when the name doesn't follow the convention, or the autofetched icon is wrong, the user clicks the icon button and either:
  - pastes an image URL, which calls `icon.fetch` with `fromImageUrl`; or
  - picks an existing subscription from a dropdown that shows each subscription with its icon, which reuses that icon (`fromSubscription`).

The drawer then sends the icon's data URI as `icon` in `subscription.start`. It does not use `iconAuto`, because the icon is already resolved and shown to the user before submitting.

**CLI and MCP:** there is no drawer, so fetching and saving are two steps:

1. Call `icon.fetch` to get an icon data URI, with one of:
   - `{"fromDomain": "netflix.com"}`
   - `{"fromImageUrl": "https://example.com/logo.png"}`
   - `{"fromSubscription": "netflix.com"}` to reuse the icon of an existing subscription, looked up by name.
2. Pass the returned data URI as `icon` to `subscription.start` or `subscription.edit`.

`iconAuto` remains as a one-step shortcut for names that follow the domain convention.

## Icon format

An icon is a 64×64 PNG, encoded as a data URI: `data:image/png;base64,<...>`. This is the value stored in the subscription's `icon` column and returned by every service function, so clients can use it directly as an image source.

64px is twice the 32px display size, so icons stay sharp on HiDPI screens.

The clients return PNG bytes. Only the service encodes them as a data URI, so the format is defined in one place.

## Errors

The service functions return a normalized set of errors, owned by the `icon` package:

- `ErrNotFound`: there is no icon for the input.
- `ErrNotAnImage`: the source responded, but not with an image the service can decode.
- `ErrFailed`: the icon could not be retrieved (network error, timeout, cancelled `ctx`, unexpected status, body too large).

The clients and the store never leak their own errors. The service wraps them into one of the sentinels above, keeping the original as the cause (`%w`) for logging, so `errors.Is(err, context.DeadlineExceeded)` still works.

`SubscriptionIcon` returns `ErrNotFound` both when no subscription has that name (`store.ErrNotFound`) and when it exists without an icon. Other store errors become `ErrFailed`.

Operations map all three errors to `icon_fetch_failed` (see [operations](./20261018-operations.md)).

## Request limits

Both clients share the same limits, so a slow or hostile server cannot hang a request or exhaust memory:

- **Timeout:** the default `*http.Client` has a 10s `Timeout`. Every method also takes a `ctx`, so adapters can cancel or shorten a request.
- **Redirects:** at most 5, through `CheckRedirect`.
- **Body size:** bodies are read through `io.LimitReader` capped at 1 MiB. A larger body returns `ErrBodyTooLarge` (mapped to `ErrFailed`).
- **Decoded size:** before a full decode, `image.DecodeConfig` reads the header; images above 4096×4096 return `ErrNotAnImage`. This guards against decompression bombs, whose small bodies decode to huge bitmaps.

## API client: FaviconDL

```go
func New(opts ...Option) *Client
func WithHTTPClient(c *http.Client) Option
func WithBaseURL(u string) Option

func (c *Client) FetchIcon(ctx context.Context, domain string) (png []byte, err error)
```

`baseURL` defaults to `https://favicondl.com`. It can be overridden for tests (`httptest.Server`).

Requests `GET {baseURL}/api/extract?url={domain}&size=64`, built with `url.Values`, never with `fmt.Sprintf`.
The `url` parameter officially accepts a bare domain (`example.com`) as well as a full URL; FaviconDL normalizes it (lowercases the host, assumes https, strips one leading `www.`).
Follows redirects: the endpoint answers with a 302 to `/api/proxy`, which serves the image.
Maps the final response status as follows:

| Status | Meaning | Result |
| --- | --- | --- |
| 200 | Icon found (from the site, or from FaviconDL's Google/DuckDuckGo fallback) | the image |
| 404 | Neither the site nor the fallback has an icon (`Upstream returned 404`; observed, not documented) | `ErrNotFound` |
| 400 | `Invalid domain`: the input cannot be read as a hostname | `ErrNotFound` |
| other, or network error | `502 Favicon extraction failed`, rate limiting, outages | `ErrFailed` |

FaviconDL [documents](https://favicondl.com/documentation.html) that a request with `size` returns "an exact square PNG", and FaviconDL does the resizing (Lanczos, transparent padding, SVG rasterization). The client still checks the body with `image.DecodeConfig`: if it is not a 64×64 PNG (for example an error page served with status 200), it returns `ErrNotAnImage`. It never decodes or re-encodes a valid response.

A value that is not domain-like (e.g. `Netflix`) gets `400 Invalid domain`, hence `ErrNotFound`.

## Image client

```go
func New(opts ...Option) *Client
func WithHTTPClient(c *http.Client) Option

func (c *Client) FetchIcon(ctx context.Context, rawURL string) (png []byte, err error)
```

Downloads an image from an arbitrary URL and normalizes it to the [icon format](#icon-format).

```go
func (c *Client) FetchIcon(ctx context.Context, rawURL string) ([]byte, error) {
	if err := validateImageURL(rawURL); err != nil {
		return nil, err
	}
	body, err := c.download(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if isIconPNG(body) {
		return body, nil
	}
	img, err := decodeImage(body)
	if err != nil {
		return nil, err
	}
	return encodePNG(squareIcon(img))
}
```

### URL policy

Only absolute `http` and `https` URLs are accepted, for the initial request and every redirect. Anything else (`file:`, `data:`, relative URLs) returns `ErrInvalidURL`, mapped to `ErrNotFound`.

Destination addresses are not restricted: loopback and private-network URLs are allowed. Subtill is a single-user app, and the REST adapter is protected by a bearer token, so whoever can call `icon.fetch` already controls the machine's data. The trade-off is that a bearer-token holder can make the server request internal URLs (SSRF). The response is only ever returned as a valid 64×64 PNG, never as raw bytes, which limits what can be read that way.

### Download

| Status | Result |
| --- | --- |
| 200 | continue with the body |
| 404 | `ErrNotFound` |
| other, or network error | `ErrFailed` |

### Format detection

The format comes from the body, not from the `Content-Type` header, which can be missing, wrong, or carry parameters. Decoders are registered with the `image` package, and `image.Decode` picks one by magic bytes.

Supported formats:

- PNG, JPEG, GIF: standard library.
- WebP: `golang.org/x/image/webp`.
- ICO: a small ICO decoder. It picks the largest entry, which may be PNG or BMP.

Anything else, including SVG and HTML, returns `ErrNotAnImage`. Rasterizing SVG would need a much heavier dependency; SVG favicons are still available through `FromDomain`, where FaviconDL rasterizes them.

### Normalization

The image is decoded once into an `image.Image`, transformed, and encoded as PNG once. A body that is already a 64×64 PNG is returned as is, without decoding.

- **Animated GIF:** the first frame is used.
- **Non-square images** are centered on a transparent square canvas, with the longer side as its size. They are never cropped or stretched.
- **Resizing** to 64×64 uses `golang.org/x/image/draw` with Catmull-Rom, for both downscaling and upscaling. Small images are upscaled and may look soft, which matches FaviconDL.

Unexported functions:

- `validateImageURL(rawURL string) error`: `ErrInvalidURL` unless http/https
- `(c *Client) download(ctx context.Context, rawURL string) (body []byte, err error)`: applies the status table and the body cap
- `isIconPNG(body []byte) bool`: reports whether body is already a 64×64 PNG
- `decodeImage(body []byte) (image.Image, error)`: checks dimensions with `DecodeConfig`, then decodes any supported format; `ErrNotAnImage` otherwise
- `squareIcon(img image.Image) image.Image`: pads to a square and resizes to 64×64
- `encodePNG(img image.Image) ([]byte, error)`

## Testing

No test touches the network.

- **Clients:** tests run against an `httptest.Server`. The FaviconDL client points at it with `WithBaseURL`. The image client is given its URL directly. Images (PNG, JPEG, GIF, WebP, ICO, SVG, oversized and non-square) and error bodies are fixtures under `testdata/`.
- **Service:** `IconService` tests use fakes of `DomainIconFetcher` and `ImageIconFetcher`. `SubscriptionIcon` runs against a real database in a temp directory, like the `store` tests.
- **Contract with FaviconDL:** the 404 status is observed, not documented. A fixture records it, and the client is re-checked against the live API when FaviconDL changes.

## Consequences

- **Privacy:** every `FromDomain` call, including each `iconAuto`, sends the subscription name to favicondl.com, and through its fallback possibly to Google or DuckDuckGo. Those services learn which subscriptions the user has. We accept this in exchange for not maintaining a scraper. `FromImageURL` and `SubscriptionIcon` contact no third party beyond the given URL.
- **Availability:** `iconAuto` depends on a free, keyless service with fair-use limits. When it fails, the operation fails with `icon_fetch_failed`; the user can retry or set the icon manually.
- **Storage:** a 64×64 PNG data URI is typically a few kilobytes per subscription, so it is fine to store in the row and return in list responses.
- **Fallback icons:** FaviconDL's Google fallback may return a generic globe for sites without an icon. It is accepted as a found icon.
- **Stale icons:** icons are never refreshed automatically; a site's rebrand shows only after the icon is fetched again.
