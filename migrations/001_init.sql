-- AIPedia starter schema placeholder.
CREATE TABLE IF NOT EXISTS schema_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO schema_meta (key, value)
VALUES ('app', 'aipedia-api')
ON CONFLICT (key) DO NOTHING;

