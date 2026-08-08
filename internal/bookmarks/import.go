package bookmarks

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ImportDir walks BOOKMARKS_DIR (ai/ and tech/) and upserts into SQLite.
func ImportDir(ctx context.Context, db *sql.DB, root string) (int, int, error) {
	root = filepath.Clean(root)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM entries`); err != nil {
		return 0, 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM categories`); err != nil {
		return 0, 0, err
	}

	entryCount := 0
	idByPath := map[string]int64{}

	ensure := func(path, slug, title, kind string, parentID sql.NullInt64, sourcePath sql.NullString) (int64, error) {
		if id, ok := idByPath[path]; ok {
			return id, nil
		}
		var id int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO categories (path, slug, title, parent_id, kind, source_path, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
			RETURNING id
		`, path, slug, title, parentID, kind, sourcePath).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("insert category %s: %w", path, err)
		}
		idByPath[path] = id
		return id, nil
	}

	nullParent := sql.NullInt64{}
	for _, top := range []string{"ai", "tech"} {
		topDir := filepath.Join(root, top)
		st, err := os.Stat(topDir)
		if err != nil || !st.IsDir() {
			continue
		}
		topID, err := ensure(top, top, TitleFromSlug(top), "folder", nullParent, sql.NullString{})
		if err != nil {
			return 0, 0, err
		}

		entries, err := os.ReadDir(topDir)
		if err != nil {
			return 0, 0, err
		}
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			full := filepath.Join(topDir, name)
			if e.IsDir() {
				ec, err := importSubdir(ctx, tx, top, name, topID, full, ensure)
				if err != nil {
					return 0, 0, err
				}
				entryCount += ec
				continue
			}
			if !strings.HasSuffix(strings.ToLower(name), ".md") {
				continue
			}
			stem := strings.TrimSuffix(name, filepath.Ext(name))
			rel := filepath.ToSlash(filepath.Join(top, name))
			leafPath := top + "/" + stem
			title := TitleFromSlug(stem)
			body, readErr := os.ReadFile(full)
			if readErr == nil {
				if parsed, perr := ParseMarkdownFile(string(body)); perr == nil && parsed.Title != "" {
					title = parsed.Title
				}
			}
			leafID, err := ensure(leafPath, stem, title, "leaf",
				sql.NullInt64{Int64: topID, Valid: true},
				sql.NullString{String: rel, Valid: true})
			if err != nil {
				return 0, 0, err
			}
			ec, err := importFile(ctx, tx, full, leafID)
			if err != nil {
				return 0, 0, err
			}
			entryCount += ec
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO schema_meta (key, value) VALUES ('bookmarks_imported_at', ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value
	`, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return len(idByPath), entryCount, nil
}

type ensureFn func(path, slug, title, kind string, parentID sql.NullInt64, sourcePath sql.NullString) (int64, error)

func importSubdir(
	ctx context.Context,
	tx *sql.Tx,
	top, dirName string,
	parentID int64,
	dirPath string,
	ensure ensureFn,
) (entryCount int, err error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}
	var mdFiles []string
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(strings.ToLower(f.Name()), ".md") {
			continue
		}
		mdFiles = append(mdFiles, f.Name())
	}
	if len(mdFiles) == 0 {
		return 0, nil
	}

	folderPath := top + "/" + dirName

	onlyAll := len(mdFiles) == 1 && strings.EqualFold(mdFiles[0], "all.md")
	if onlyAll {
		rel := filepath.ToSlash(filepath.Join(top, dirName, "all.md"))
		title := TitleFromSlug(dirName)
		body, err := os.ReadFile(filepath.Join(dirPath, "all.md"))
		if err != nil {
			return 0, err
		}
		parsed, err := ParseMarkdownFile(string(body))
		if err != nil {
			return 0, err
		}
		if parsed.Title != "" {
			title = parsed.Title
		}
		leafID, err := ensure(folderPath, dirName, title, "leaf",
			sql.NullInt64{Int64: parentID, Valid: true},
			sql.NullString{String: rel, Valid: true})
		if err != nil {
			return 0, err
		}
		return insertRows(ctx, tx, leafID, parsed)
	}

	folderID, err := ensure(folderPath, dirName, TitleFromSlug(dirName), "folder",
		sql.NullInt64{Int64: parentID, Valid: true}, sql.NullString{})
	if err != nil {
		return 0, err
	}

	totalEntries := 0
	for _, md := range mdFiles {
		stem := strings.TrimSuffix(md, filepath.Ext(md))
		leafPath := folderPath + "/" + stem
		rel := filepath.ToSlash(filepath.Join(top, dirName, md))
		full := filepath.Join(dirPath, md)
		body, err := os.ReadFile(full)
		if err != nil {
			return 0, err
		}
		parsed, err := ParseMarkdownFile(string(body))
		if err != nil {
			return 0, err
		}
		title := TitleFromSlug(stem)
		if parsed.Title != "" {
			title = parsed.Title
		}
		leafID, err := ensure(leafPath, stem, title, "leaf",
			sql.NullInt64{Int64: folderID, Valid: true},
			sql.NullString{String: rel, Valid: true})
		if err != nil {
			return 0, err
		}
		ec, err := insertRows(ctx, tx, leafID, parsed)
		if err != nil {
			return 0, err
		}
		totalEntries += ec
	}
	return totalEntries, nil
}

func importFile(ctx context.Context, tx *sql.Tx, full string, categoryID int64) (int, error) {
	body, err := os.ReadFile(full)
	if err != nil {
		return 0, err
	}
	parsed, err := ParseMarkdownFile(string(body))
	if err != nil {
		return 0, err
	}
	return insertRows(ctx, tx, categoryID, parsed)
}

func insertRows(ctx context.Context, tx *sql.Tx, categoryID int64, parsed ParsedTable) (int, error) {
	n := 0
	for i, row := range parsed.Rows {
		ef := MapRow(row)
		if ef.Name == "" {
			continue
		}
		attrs, _ := json.Marshal(ef.Attrs)
		_, err := tx.ExecContext(ctx, `
			INSERT INTO entries (
				category_id, name, name_url, logo_url, owner_name, owner_url,
				website_url, description, origin, free_plan, paid_plan, links, attrs, source_row, updated_at
			) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,datetime('now'))
		`, categoryID, ef.Name, nullStr(ef.NameURL), nullStr(ef.LogoURL), nullStr(ef.OwnerName), nullStr(ef.OwnerURL),
			nullStr(ef.WebsiteURL), nullStr(ef.Description), nullStr(ef.Origin), nullStr(ef.FreePlan), nullStr(ef.PaidPlan),
			nullStr(ef.Links), attrs, i+1)
		if err != nil {
			return n, fmt.Errorf("insert entry %q: %w", ef.Name, err)
		}
		n++
	}
	return n, nil
}

func nullStr(s string) sql.NullString {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
