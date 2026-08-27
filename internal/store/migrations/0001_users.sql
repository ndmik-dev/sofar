-- The single-user insurance: every later table carries user_id so growing from
-- one person to a handful is an evening, not a data migration.
CREATE TABLE users (
    id         INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL,
    created_at INTEGER NOT NULL
);

INSERT INTO users (id, name, created_at) VALUES (1, 'me', strftime('%s', 'now'));
