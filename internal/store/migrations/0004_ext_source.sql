-- Books, games and podcasts come from catalogs of their own, so media needs a
-- generic external id alongside the TMDB pair.
ALTER TABLE media ADD COLUMN ext_id TEXT;

CREATE UNIQUE INDEX media_ext ON media(source, ext_id) WHERE ext_id IS NOT NULL;
