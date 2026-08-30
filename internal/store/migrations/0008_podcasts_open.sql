-- A podcast has no finish line, so "хочу / у процесі / завершив" never had an
-- honest answer for it. It becomes an open counter instead: one press is one
-- episode listened, and there is still no total, which is the whole point.
UPDATE type_settings SET depth = 'units', step = 1 WHERE kind = 'podcast';

UPDATE media SET unit = 'episode', total_units = NULL WHERE kind = 'podcast';

-- A podcast marked "завершив" was never finished; it goes back to the list it
-- actually belongs on.
UPDATE entry
SET status = 'active', finished_at = NULL
WHERE status = 'done'
  AND media_id IN (SELECT id FROM media WHERE kind = 'podcast');
