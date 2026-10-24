# Review of External APIs

## Favicon APIs

There is no single **official, standardized** “favicon API” (no IETF/W3C spec for “give me a 32×32 PNG for any domain”). What people use falls into three buckets: **documented third-party proxies**, **undocumented big-provider endpoints**, and **self-hosted / direct fetch** (best for privacy and format control).

Below is a practical map against your criteria: **free, no auth, preferably 32×32 PNG**.

---

### Best fit for “32×32 PNG, documented, no key”

| Service | Example | 32×32 PNG? | Docs |
|--------|---------|------------|------|
| **FaviconDL** | `https://favicondl.com/api/extract?url=github.com&size=32` | Yes — `size` yields square PNG | [Documentation](https://favicondl.com/documentation.html) |
| **Fav.dog** | `https://fav.dog/github.com?s=32` | Yes — resize to PNG (`s` = 16, 32, 48, 64, 128, 256) | [API docs](https://fav.dog/api) |
| **FetchFavicon** | `https://fetchfavicon.com/i/github.com?size=32&format=png` | Yes | [API docs](https://fetchfavicon.com/api) |
| **favicon.run** | `https://favicon.run/favicon?domain=github.com&sz=32` | `sz` 16–512; CORS `*` | [Favicon API](https://favicon.run/favicon) (overview also in [llms.txt](https://favicon.run/llms.txt)) |

**Caveats**

- **FetchFavicon** documents a **free tier** (~3,000 requests/month, 60/min) — fine for small apps, not for heavy crawling. See their [API docs](https://fetchfavicon.com/api).
- **FaviconDL** asks for fair use and edge caching; they document upscaling when the source is smaller than requested ([docs](https://favicondl.com/documentation.html)).
- None of these guarantee the *semantic* “official” icon every time; they resolve what the site publishes (or fallbacks).

---

### Widely used but **undocumented / unsupported**

These are **not** products with stable SLAs. Many production apps use them anyway; Google/Chromium folks have described the Google URL as **not a supported public API**.

| Service | Example | 32×32 PNG? | “Docs” |
|--------|---------|------------|--------|
| **Google (legacy redirect)** | `https://www.google.com/s2/favicons?domain=example.com&sz=32` | Usually PNG; `sz` is a *request*, actual pixels may be smaller | No official docs; community reference e.g. [Logo.dev summary](https://www.logo.dev/docs/google-favicon-api), [Chromium extensions discussion](https://groups.google.com/a/chromium.org/g/chromium-extensions/c/C136OdBtXpI) |
| **Google (gstatic `faviconV2`)** | `https://t2.gstatic.com/faviconV2?client=SOCIAL&type=FAVICON&fallback_opts=TYPE,SIZE,URL&url=https://example.com&size=32` | Often PNG at requested size when available | Undocumented; parameters inferred from redirects (e.g. [Stack Overflow thread](https://stackoverflow.com/questions/72244038/need-t2-gstatic-url-parameters-for-web-scraping)) |
| **DuckDuckGo** | `https://icons.duckduckgo.com/ip3/example.com.ico` | **Not** guaranteed 32×32 or PNG (often 16–48px; extension `.ico` but body may be PNG) | **No public API spec**; privacy context: [DDG favicons help](https://duckduckgo.com/duckduckgo-help-pages/privacy/favicons); endpoint visible in [DDG iOS source](https://github.com/duckduckgo/iOS/blob/main/Core/AppUrls.swift) |

**Privacy note:** Your users’ clients hit **Google/DDG**, which learn **which domains** you display. DDG’s help page explains they proxy favicons for search UX and emphasize no tracking in their product model — still a third party in the path.

---

### Aggregators (one URL, multiple backends)

| Service | Example | 32×32 PNG? | Docs |
|--------|---------|------------|------|
| **unavatar.io** | `https://unavatar.io/google/example.com` or `https://unavatar.io/duckduckgo/example.com` | Depends on chosen provider; no dedicated `size=32` on all routes | [unavatar docs](https://unavatar.io/docs) |
| **Favicon.im** | `https://a.favicon.im/github.com` | ~32px default; `?larger=true` up to ~256px; format varies | [API page](https://favicon.im/api) |

Good when you want **provider choice** or a **domain logo** pipeline (unavatar tries BIMI → DDG → Google → Microlink for `/domain/...` per [docs](https://unavatar.io/docs)).

---

### Simple URL, weaker format guarantees

| Service | Example | Docs |
|--------|---------|------|
| **Icon Horse** | `https://icon.horse/icon/github.com` | [Usage / parameters](https://iconhorse.com/usage), [Privacy](https://icon.horse/privacy) |

Free tier works without a key for basic `hostname`/`uri`. **Exact 32×32 PNG** and strict size behavior are largely **Pro** (`size`, `ignore_other_sizes`, etc. per [usage](https://iconhorse.com/usage)). Blog mentions `?size=32` but the canonical parameter doc lists named sizes for Pro — treat **32×32** as “maybe closest available,” not guaranteed on free tier.

---

### Most privacy-respecting: **no public third-party API**

| Approach | Why |
|----------|-----|
| **Your backend fetches** the site’s `link rel="icon"` / `apple-touch-icon` / `/favicon.ico`, normalize to 32×32 PNG yourself | Only your infra talks to target sites; you control retention and logging. |
| **Self-host [besticon](https://github.com/mat/besticon)** | Open-source Go service; API like `/icon?url=example.com&size=32..50..100&formats=png,ico`. Docs: [README](https://github.com/mat/besticon/blob/master/Readme.markdown), [pkg.go.dev](https://pkg.go.dev/github.com/mat/besticon/v3). Public demo exists but self-hosting is the privacy story. |

There is also **no auth “standard”** for reading another site’s favicon — you’re always either fetching the origin or trusting a proxy.

---

### Quick recommendations

1. **Need documented `size=32` PNG, no key, minimal setup:** [FaviconDL](https://favicondl.com/documentation.html) or [Fav.dog](https://fav.dog/api).
2. **Need CORS for browser `<img>` hotlinking:** [favicon.run](https://favicon.run/favicon) (confirm behavior under your load).
3. **Okay with undocumented Google behavior:** `www.google.com/s2/favicons?domain=…&sz=32` — cache aggressively; expect changes.
4. **Privacy-first / production without leaking domains to Google:** self-host **besticon** or resolve icons in your own service.
5. **Strict “always exactly 32×32 PNG”:** only services that **rasterize/resize** (FaviconDL, Fav.dog, FetchFavicon with `format=png`) — and accept upscaling blur when the source is tiny.

If you say whether icons are loaded **in the browser** (CORS/hotlink) vs **server-side only**, and approximate volume, narrowing to one or two options is straightforward.

## Currency Conversion APIs

There is no single standard “convert any currency to any currency” registry API. In practice, services return **rates from one base ISO 4217 code to many others**; **A → B** is either a dedicated convert endpoint or `rate(A→B) = rate(base→B) / rate(base→A)`. “Current” usually means **latest published daily reference rates**, not live trading ticks.

Below are **free, no API key** options that support **latest + historical**, with doc links. I also note common **free-but-key-required** services people still compare against.

---

### Strongest no-key options (current + historical)

#### 1. Frankfurter (recommended default)

- **Latest:** `GET https://api.frankfurter.dev/v2/rates?base=USD&quotes=EUR,GBP`
- **Historical (one day):** `?date=1999-01-04`
- **Time series:** `?from=2024-01-01&to=2024-12-31&base=USD&quotes=EUR`
- **Coverage:** ~200+ currencies, blended official/central-bank sources; history back to **1948** (varies by pair/provider)
- **Auth:** none
- **Any→any:** any `base` + `quotes`; cross via two quotes from same base/date
- **Docs:** [Frankfurter docs (v2)](https://www.frankfurter.dev/docs/) · [Site / FAQ](https://frankfurter.dev/) · [GitHub (self-host)](https://github.com/lineofflight/frankfurter)
- **Privacy:** FAQ states no logging of personal data / request URLs on the public instance ([FAQ on frankfurter.dev](https://frankfurter.dev/))

Legacy v1 paths still work: [v1 docs](https://frankfurter.dev/v1/).

---

#### 2. fawazahmed0 / exchange-api (static JSON on CDN)

Not a classic REST “convert” API; **daily JSON files** per date and base currency.

- **Latest (USD base example):**
  `https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/usd.json`
- **Historical:** replace `@latest` with `@2024-03-06` (or use `https://2024-03-06.currency-api.pages.dev/v1/currencies/usd.json`)
- **Any→any:** pick any supported code as base in the path; convert using two rates vs same base or direct key in JSON
- **Auth:** none · **License:** CC0
- **Docs:** [exchange-api README](https://github.com/fawazahmed0/exchange-api/blob/main/README.md) · [npm package pointer](https://www.npmjs.com/package/@fawazahmed0/currency-api)

**Caveats:** daily updates; includes some crypto/metals; reliability = CDN + published files (use jsDelivr + Cloudflare fallback as documented).

---

#### 3. The Rates API (ECB reference rates, small currency set)

Thin wrapper around **ECB euro foreign exchange reference rates** with cross-rates for arbitrary bases.

- **Latest:** `GET https://theratesapi.com/api/latest?base=USD&symbols=GBP,EUR`
- **Historical:** `GET https://theratesapi.com/api/2025-06-02?base=USD&symbols=GBP`
- **History from:** 1999-01-04 (working days; weekends/holidays roll to previous business day)
- **Currencies:** ~**30** ECB majors (not “every” ISO currency)
- **Auth:** none · **CORS:** open
- **Docs:** [Documentation](https://theratesapi.com/documentation/) · [Home](https://theratesapi.com/)

Good when you only need major fiat and want a very small API surface.

---

#### 4. ECB Data Portal (official source, no key)

Direct **SDMX** access to dataset **EXR** (e.g. daily USD per EUR spot reference).

- **Example:**
  `https://data-api.ecb.europa.eu/service/data/EXR/D.USD.EUR.SP00.A?format=jsondata&startPeriod=2024-01-01&endPeriod=2024-12-31`
- **All currencies vs EUR:** key like `D..EUR.SP00.A`
- **Any→any:** compute crosses from EUR legs (same as Frankfurter/Rates API conceptually)
- **Auth:** none
- **Docs:** [Data API help](https://data.ecb.europa.eu/help/api/data) · [EXR structure](https://data.ecb.europa.eu/data/datasets/EXR/structure) · [Content negotiation](https://data.ecb.europa.eu/help/api/content-negotiation)

Best for **compliance/official ECB numbers**; worst for **developer ergonomics** (SDMX keys, XML/JSON shapes).

---

### No-key but limited (know the gaps)

| Service | Latest | Historical | Any→any | Docs |
|--------|--------|------------|---------|------|
| **ExchangeRate-API Open Access** | `GET https://open.er-api.com/v6/latest/USD` | **No** on open endpoint (historical needs keyed v6) | Yes via `rates` map | [Open access docs](https://www.exchangerate-api.com/docs/free) · [Terms](https://www.exchangerate-api.com/terms) |
| **currency-converter.live** | `/api/v1/rates`, `/convert` | `/api/v1/timeseries` (pair + `days`) | Yes | [API page](https://currency-converter.live/en/api) |

**ExchangeRate-API open access:** attribution required; rate-limited (429, ~20 min cooldown); **once-daily** refresh; caching encouraged.

**currency-converter.live:** asks for attribution and caching; terms are lighter than a central bank—verify fit for accounting use.

---

### Used to be no-key (now requires auth)

| Service | Status | Docs |
|--------|--------|------|
| **exchangerate.host** (APILayer) | API key required; free tier ~**100 req/month** | [Site](https://exchangerate.host/) · [Pricing](https://exchangerate.host/pricing) |
| Old **Formicka** `api.exchangerate.host` fork | Community project **dead**; do not rely on old tutorials | [Issue #245](https://github.com/Formicka/exchangerate.host/issues/245) |

---

### Free with signup (not “no authentication”, but common)

If you can accept a key, these often appear in comparisons:

| Provider | Free tier (typical) | Historical | Any base? | Docs |
|----------|---------------------|------------|-----------|------|
| **ExchangeRate-API** (keyed v6) | ~1.5k req/month | Yes (paid tiers for more) | Yes | [Docs hub](https://www.exchangerate-api.com/docs) |
| **Open Exchange Rates** | 1k/month | Limited on free | **USD base only** on free | [openexchangerates.org](https://openexchangerates.org/) |
| **Fixer** (APILayer family) | ~100/month | Varies by plan | Check current plan | [fixer.io](https://fixer.io/) |

---

### How to read “any to any” and “current”

1. **Any→any:** Request `base=A` and read `quotes=B`, or fetch both legs against EUR/USD and divide.
2. **Current:** For Frankfurter/ECB/Rates API, “latest” is **today’s or last business day’s official daily rate**, not a streaming forex quote.
3. **Historical depth:** Frankfurter and fawazahmed0 go wide on currencies; ECB/Rates API go deep on **majors** with official methodology.
4. **Privacy:** Keyless third parties still see **which bases/dates you request** unless you **self-host** ([Frankfurter](https://github.com/lineofflight/frankfurter)) or **bulk-download/cache** ECB/Frankfurter datasets.

---

### Practical picks

| Need | Suggestion |
|------|------------|
| **No key, latest + history + many currencies** | [Frankfurter v2](https://www.frankfurter.dev/docs/) |
| **No key, static files, easy caching** | [fawazahmed0 exchange-api](https://github.com/fawazahmed0/exchange-api/blob/main/README.md) |
| **No key, ~30 majors, simple REST** | [The Rates API](https://theratesapi.com/documentation/) |
| **Official ECB only** | [ECB Data API](https://data.ecb.europa.eu/help/api/data) |
| **No key but historical required** | Skip ExchangeRate-API open access; use Frankfurter or fawazahmed0 |

If you share whether subtill needs **all ISO currencies** or only **billing majors**, and whether rates are for **display** vs **ledger/tax**, narrowing to one provider (and a caching strategy) is straightforward.
