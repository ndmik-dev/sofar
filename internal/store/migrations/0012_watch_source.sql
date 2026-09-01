-- The watch moved from one publisher's shop to a shop that carries all of them.
-- The source is a label; polling never read it, so this is only tidiness.
UPDATE watch SET source = 'comicsmania' WHERE source = 'nashaidea';
