# Icon Service

Exposes three public methods:
- func FetchFromDomain(domain string) (icon []byte, err error) - wrapper for faviconDL.FetchIcon
- func FetchFromURL(url string) (icon []byte, err error) - wrapper for HttpImageClient.FetchImage
- func FetchFromSubscriptionId(subscriptionId string) (icon []byte, err error) -- wrapper for storage.Subscription.GetByName.Icon

All public methods return a normalized set of errors:
- ErrNotFound: icon not found
- ErrFailed: request failed

## Api Client: FaviconDL

Makes http request to `ftm.Sprintf("https://favicondl.com/api/extract?url=%s&size=32", domain)`. Verifies response status, converts body to base64 and returns it.

Public methods:
- func FetchIcon(domain string) (icon []byte, err error)

## Http Image Client

- func FetchImage(url string) (image []byte, err error)
Makes http request to `url`.
If response status is not 200, returns `ErrRequestFailed`.

If content type is image/png, returns the image as is.
If content type is image/jpeg (or any other supported image format), converts image to png.
If content type is not an image, returns `ErrNotAnImage`.

Checks image dimensions and ensures width and height are both 32px.
If not, passes image to `scaleImage` to resize to 32px.

Finally, converts the image to base64 and returns it.

Public methods:
- func FetchIconFromURL(url string) (image []byte, err error)

Private methods:
- func fetchImage(url string) (image []byte, err error)
- func convertImage(image []byte, contentType string) (image []byte, err error) -- converts image from contentType to png
- func scaleImage(image []byte) (scaledImage []byte, err error) -- detects image dimensions and scales image to 32px if necessary
