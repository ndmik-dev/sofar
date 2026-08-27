-- Deleting is undoable like every other action, so the row is only marked.
-- A nightly purge of anything older than 30 days lands with the cron in M9.
ALTER TABLE entry ADD COLUMN deleted_at INTEGER;

CREATE INDEX entry_live ON entry(user_id, status, updated_at DESC) WHERE deleted_at IS NULL;
