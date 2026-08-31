-- Podcasts are gone. Entries are soft-deleted rather than dropped: that is how
-- deletion works everywhere else here, and the nightly purge clears them after
-- thirty days, which leaves a month to change your mind.
UPDATE entry
SET deleted_at = CAST(strftime('%s','now') AS INTEGER)
WHERE deleted_at IS NULL
  AND media_id IN (SELECT id FROM media WHERE kind = 'podcast');

DELETE FROM type_settings WHERE kind = 'podcast';

-- Manga is read in chapters and often has no end, which the open counter
-- already handles: leave the total empty and it just counts.
INSERT INTO type_settings (user_id, kind, depth, step) VALUES (1, 'manga', 'units', 1)
ON CONFLICT (user_id, kind) DO NOTHING;
