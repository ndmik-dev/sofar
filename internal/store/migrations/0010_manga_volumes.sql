-- Manga is bought and read by the volume; chapter numbering belongs to the
-- scanlation world, not to a shelf of printed books.
UPDATE media SET unit = 'volume' WHERE kind = 'manga' AND unit = 'chapter';
