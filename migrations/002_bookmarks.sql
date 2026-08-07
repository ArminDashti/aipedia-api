-- Bookmarks catalog: nested categories + leaf entries.

CREATE TABLE IF NOT EXISTS categories (
    id          BIGSERIAL PRIMARY KEY,
    path        TEXT NOT NULL UNIQUE,
    slug        TEXT NOT NULL,
    title       TEXT NOT NULL,
    parent_id   BIGINT REFERENCES categories (id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('folder', 'leaf')),
    source_path TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_categories_parent_id ON categories (parent_id);
CREATE INDEX IF NOT EXISTS idx_categories_kind ON categories (kind);

CREATE TABLE IF NOT EXISTS entries (
    id           BIGSERIAL PRIMARY KEY,
    category_id  BIGINT NOT NULL REFERENCES categories (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    name_url     TEXT,
    logo_url     TEXT,
    owner_name   TEXT,
    owner_url    TEXT,
    website_url  TEXT,
    description  TEXT,
    origin       TEXT,
    free_plan    TEXT,
    paid_plan    TEXT,
    links        TEXT,
    attrs        JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_row   INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (category_id, name)
);

CREATE INDEX IF NOT EXISTS idx_entries_category_id ON entries (category_id);
CREATE INDEX IF NOT EXISTS idx_entries_name ON entries (name);

INSERT INTO schema_meta (key, value)
VALUES ('bookmarks_schema', '002')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
