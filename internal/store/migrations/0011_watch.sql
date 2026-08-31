-- What a person is waiting for. One watch per entry: it is the same series,
-- however many volumes the publisher eventually prints.
CREATE TABLE watch (
    entry_id   INTEGER PRIMARY KEY REFERENCES entry(id) ON DELETE CASCADE,
    source     TEXT    NOT NULL,
    query      TEXT    NOT NULL,
    last_vol   INTEGER NOT NULL DEFAULT 0,
    last_title TEXT,
    last_url   TEXT,
    found_at   INTEGER,
    checked_at INTEGER,
    created_at INTEGER NOT NULL
);
