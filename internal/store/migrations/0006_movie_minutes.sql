-- A film used to be one tick. The useful position inside it is the minute you
-- stopped at, which TMDB already gives us as the runtime.
UPDATE media
SET unit = 'minute', total_units = runtime_min
WHERE kind = 'movie' AND unit = 'none' AND COALESCE(runtime_min, 0) > 0;

-- A film already finished must not read as unwatched once the single tick
-- becomes a hundred minutes. position is a cache of the progress log, so the
-- correction is written to the log rather than straight into the column.
INSERT INTO progress (entry_id, from_pos, to_pos, at, source)
SELECT e.id, e.position, m.total_units, e.updated_at, 'setup'
FROM entry e
JOIN media m ON m.id = e.media_id
WHERE m.kind = 'movie' AND m.unit = 'minute' AND m.total_units IS NOT NULL
  AND e.status = 'done' AND e.deleted_at IS NULL
  AND e.position < m.total_units;

UPDATE entry
SET position = (SELECT total_units FROM media WHERE id = entry.media_id)
WHERE id IN (
  SELECT e.id FROM entry e
  JOIN media m ON m.id = e.media_id
  WHERE m.kind = 'movie' AND m.unit = 'minute' AND m.total_units IS NOT NULL
    AND e.status = 'done' AND e.deleted_at IS NULL
    AND e.position < m.total_units
);
