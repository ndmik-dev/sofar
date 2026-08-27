CREATE TABLE media (
    id           INTEGER PRIMARY KEY,
    kind         TEXT    NOT NULL,
    source       TEXT    NOT NULL,
    tmdb_type    TEXT,
    tmdb_id      INTEGER,
    title        TEXT    NOT NULL,
    title_orig   TEXT,
    year         INTEGER,
    overview     TEXT,
    poster_path  TEXT,
    runtime_min  INTEGER,
    total_units  INTEGER,
    unit         TEXT    NOT NULL,
    airing       TEXT,
    refreshed_at INTEGER,
    created_at   INTEGER NOT NULL
);

CREATE UNIQUE INDEX media_tmdb ON media(tmdb_type, tmdb_id) WHERE tmdb_id IS NOT NULL;

CREATE TABLE unit (
    media_id    INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    idx         INTEGER NOT NULL,
    season      INTEGER,
    number      INTEGER,
    title       TEXT,
    air_date    TEXT,
    runtime_min INTEGER,
    PRIMARY KEY (media_id, idx)
) WITHOUT ROWID;

CREATE TABLE entry (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL DEFAULT 1 REFERENCES users(id),
    media_id    INTEGER NOT NULL REFERENCES media(id),
    run         INTEGER NOT NULL DEFAULT 1,
    status      TEXT    NOT NULL,
    position    INTEGER NOT NULL DEFAULT 0,
    rating      INTEGER,
    note        TEXT,
    started_at  INTEGER,
    finished_at INTEGER,
    updated_at  INTEGER NOT NULL,
    created_at  INTEGER NOT NULL,
    UNIQUE (user_id, media_id, run)
);

CREATE INDEX entry_list ON entry(user_id, status, updated_at DESC);

CREATE TABLE progress (
    id        INTEGER PRIMARY KEY,
    entry_id  INTEGER NOT NULL REFERENCES entry(id) ON DELETE CASCADE,
    from_pos  INTEGER NOT NULL,
    to_pos    INTEGER NOT NULL,
    at        INTEGER NOT NULL,
    source    TEXT    NOT NULL DEFAULT 'manual',
    undone_at INTEGER
);

CREATE INDEX progress_entry ON progress(entry_id, at DESC);
CREATE INDEX progress_live  ON progress(at) WHERE undone_at IS NULL;

CREATE TABLE type_settings (
    user_id INTEGER NOT NULL REFERENCES users(id),
    kind    TEXT    NOT NULL,
    depth   TEXT    NOT NULL DEFAULT 'units',
    step    INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (user_id, kind)
) WITHOUT ROWID;

INSERT INTO type_settings (user_id, kind, depth, step) VALUES
    (1, 'show',    'units',  1),
    (1, 'anime',   'units',  1),
    (1, 'movie',   'status', 1),
    (1, 'book',    'units', 10),
    (1, 'game',    'status', 1),
    (1, 'podcast', 'status', 1);

CREATE TABLE streak_freeze (
    user_id INTEGER NOT NULL REFERENCES users(id),
    day     TEXT    NOT NULL,
    PRIMARY KEY (user_id, day)
) WITHOUT ROWID;
