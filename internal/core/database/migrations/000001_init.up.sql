CREATE TABLE subscriptions (
    id      TEXT NOT NULL PRIMARY KEY CHECK (id <> ''),
    name    TEXT NOT NULL COLLATE NOCASE UNIQUE,
    "group" TEXT,
    notes   TEXT,
    icon    TEXT,
    url     TEXT
);

CREATE TABLE tags (
    id              TEXT NOT NULL PRIMARY KEY CHECK (id <> ''),
    subscription_id TEXT NOT NULL REFERENCES subscriptions (id) ON DELETE CASCADE,
    tag             TEXT NOT NULL COLLATE NOCASE,
    UNIQUE (subscription_id, tag)
);

CREATE TABLE billing_periods (
    id               TEXT    NOT NULL PRIMARY KEY CHECK (id <> ''),
    subscription_id  TEXT    NOT NULL REFERENCES subscriptions (id) ON DELETE CASCADE,
    billing_currency TEXT    NOT NULL,
    billing_cost     TEXT    NOT NULL,
    cost             TEXT    NOT NULL,
    frequency        INTEGER NOT NULL CHECK (frequency > 0),
    start_on         TEXT    NOT NULL,
    end_on           TEXT,
    CHECK (end_on IS NULL OR end_on > start_on)
);

CREATE TABLE reminders (
    id              TEXT    NOT NULL PRIMARY KEY CHECK (id <> ''),
    subscription_id TEXT    NOT NULL REFERENCES subscriptions (id) ON DELETE CASCADE,
    action          TEXT    NOT NULL CHECK (action IN ('cancel', 'renew', 'custom')),
    custom_note     TEXT,
    trigger_on      TEXT    NOT NULL,
    is_triggered    INTEGER NOT NULL DEFAULT 0 CHECK (is_triggered IN (0, 1))
);

CREATE TABLE exchange_rates (
    currency    TEXT NOT NULL,
    rate        TEXT NOT NULL,
    exchange_on TEXT NOT NULL,
    UNIQUE (currency, exchange_on)
);

CREATE TABLE preferences (
    id                       INTEGER PRIMARY KEY CHECK (id = 1),
    currency                 TEXT    NOT NULL DEFAULT 'USD',
    currency_conversion_mode TEXT    NOT NULL DEFAULT 'approximate' CHECK (currency_conversion_mode IN ('approximate', 'live'))
);

CREATE INDEX subscriptions_name_idx ON subscriptions ("name");
CREATE INDEX billing_periods_subscription_id_idx ON billing_periods (subscription_id);
CREATE INDEX reminders_trigger_on_idx ON reminders (trigger_on);
