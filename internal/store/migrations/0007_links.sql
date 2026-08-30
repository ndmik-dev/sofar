-- Links belong to the entry, not to the media: where you watch something is
-- yours, not a property of the show.
CREATE TABLE link (
    id         INTEGER PRIMARY KEY,
    entry_id   INTEGER NOT NULL REFERENCES entry(id) ON DELETE CASCADE,
    url        TEXT    NOT NULL,
    label      TEXT    NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX link_entry ON link(entry_id, id);
