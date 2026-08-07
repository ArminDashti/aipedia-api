package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type categoryDTO struct {
	ID          int64   `json:"id"`
	Path        string  `json:"path"`
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	Kind        string  `json:"kind"`
	ParentPath  *string `json:"parentPath,omitempty"`
	SourcePath  *string `json:"sourcePath,omitempty"`
	ChildCount  int     `json:"childCount"`
	EntryCount  int     `json:"entryCount"`
}

type entryDTO struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	NameURL     *string           `json:"nameUrl,omitempty"`
	LogoURL     *string           `json:"logoUrl,omitempty"`
	OwnerName   *string           `json:"ownerName,omitempty"`
	OwnerURL    *string           `json:"ownerUrl,omitempty"`
	WebsiteURL  *string           `json:"websiteUrl,omitempty"`
	Description *string           `json:"description,omitempty"`
	Origin      *string           `json:"origin,omitempty"`
	FreePlan    *string           `json:"freePlan,omitempty"`
	PaidPlan    *string           `json:"paidPlan,omitempty"`
	Links       *string           `json:"links,omitempty"`
	Attrs       map[string]any    `json:"attrs,omitempty"`
}

// ListCategories returns root categories or children of ?parent=path.
func (h *Handlers) ListCategories(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	parent := strings.Trim(strings.TrimSpace(c.Query("parent")), "/")
	var rows *sql.Rows
	var err error
	if parent == "" {
		rows, err = h.db.QueryContext(ctx, `
			SELECT c.id, c.path, c.slug, c.title, c.kind, c.source_path,
				(SELECT COUNT(*) FROM categories ch WHERE ch.parent_id = c.id),
				(SELECT COUNT(*) FROM entries e WHERE e.category_id = c.id)
			FROM categories c
			WHERE c.parent_id IS NULL
			ORDER BY c.path
		`)
	} else {
		rows, err = h.db.QueryContext(ctx, `
			SELECT c.id, c.path, c.slug, c.title, c.kind, c.source_path,
				(SELECT COUNT(*) FROM categories ch WHERE ch.parent_id = c.id),
				(SELECT COUNT(*) FROM entries e WHERE e.category_id = c.id)
			FROM categories c
			JOIN categories p ON p.id = c.parent_id
			WHERE p.path = $1
			ORDER BY c.path
		`, parent)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	list, err := scanCategories(rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "scan failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"categories": list})
}

// CategoryByPath handles /api/categories/*path including /children and /entries suffixes.
func (h *Handlers) CategoryByPath(c *gin.Context) {
	raw := strings.TrimPrefix(c.Param("path"), "/")
	raw = strings.Trim(raw, "/")
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}

	mode := "get"
	path := raw
	if strings.HasSuffix(raw, "/children") {
		mode = "children"
		path = strings.TrimSuffix(raw, "/children")
		path = strings.Trim(path, "/")
	} else if strings.HasSuffix(raw, "/entries") {
		mode = "entries"
		path = strings.TrimSuffix(raw, "/entries")
		path = strings.Trim(path, "/")
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	switch mode {
	case "children":
		h.listChildren(c, ctx, path)
	case "entries":
		h.listEntries(c, ctx, path)
	default:
		h.getCategory(c, ctx, path)
	}
}

func (h *Handlers) getCategory(c *gin.Context, ctx context.Context, path string) {
	row := h.db.QueryRowContext(ctx, `
		SELECT c.id, c.path, c.slug, c.title, c.kind, c.source_path, p.path,
			(SELECT COUNT(*) FROM categories ch WHERE ch.parent_id = c.id),
			(SELECT COUNT(*) FROM entries e WHERE e.category_id = c.id)
		FROM categories c
		LEFT JOIN categories p ON p.id = c.parent_id
		WHERE c.path = $1
	`, path)

	dto, err := scanCategoryRow(row)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	c.JSON(http.StatusOK, dto)
}

func (h *Handlers) listChildren(c *gin.Context, ctx context.Context, path string) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT c.id, c.path, c.slug, c.title, c.kind, c.source_path,
			(SELECT COUNT(*) FROM categories ch WHERE ch.parent_id = c.id),
			(SELECT COUNT(*) FROM entries e WHERE e.category_id = c.id)
		FROM categories c
		JOIN categories p ON p.id = c.parent_id
		WHERE p.path = $1
		ORDER BY c.path
	`, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()
	list, err := scanCategories(rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "scan failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"categories": list})
}

func (h *Handlers) listEntries(c *gin.Context, ctx context.Context, path string) {
	q := strings.TrimSpace(c.Query("q"))
	var rows *sql.Rows
	var err error
	if q == "" {
		rows, err = h.db.QueryContext(ctx, `
			SELECT e.id, e.name, e.name_url, e.logo_url, e.owner_name, e.owner_url,
				e.website_url, e.description, e.origin, e.free_plan, e.paid_plan, e.links, e.attrs
			FROM entries e
			JOIN categories c ON c.id = e.category_id
			WHERE c.path = $1
			ORDER BY e.name
		`, path)
	} else {
		like := "%" + q + "%"
		rows, err = h.db.QueryContext(ctx, `
			SELECT e.id, e.name, e.name_url, e.logo_url, e.owner_name, e.owner_url,
				e.website_url, e.description, e.origin, e.free_plan, e.paid_plan, e.links, e.attrs
			FROM entries e
			JOIN categories c ON c.id = e.category_id
			WHERE c.path = $1 AND (
				e.name ILIKE $2 OR COALESCE(e.description, '') ILIKE $2 OR COALESCE(e.owner_name, '') ILIKE $2
			)
			ORDER BY e.name
		`, path, like)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	list, err := scanEntries(rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "scan failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": list})
}

// SearchEntries searches all entries with ?q=.
func (h *Handlers) SearchEntries(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	like := "%" + q + "%"
	rows, err := h.db.QueryContext(ctx, `
		SELECT e.id, e.name, e.name_url, e.logo_url, e.owner_name, e.owner_url,
			e.website_url, e.description, e.origin, e.free_plan, e.paid_plan, e.links, e.attrs
		FROM entries e
		WHERE e.name ILIKE $1 OR COALESCE(e.description, '') ILIKE $1 OR COALESCE(e.owner_name, '') ILIKE $1
		ORDER BY e.name
		LIMIT 100
	`, like)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()
	list, err := scanEntries(rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "scan failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": list})
}

func scanCategories(rows *sql.Rows) ([]categoryDTO, error) {
	list := make([]categoryDTO, 0)
	for rows.Next() {
		var dto categoryDTO
		var source sql.NullString
		if err := rows.Scan(&dto.ID, &dto.Path, &dto.Slug, &dto.Title, &dto.Kind, &source, &dto.ChildCount, &dto.EntryCount); err != nil {
			return nil, err
		}
		if source.Valid {
			dto.SourcePath = &source.String
		}
		list = append(list, dto)
	}
	return list, rows.Err()
}

func scanCategoryRow(row *sql.Row) (categoryDTO, error) {
	var dto categoryDTO
	var source, parent sql.NullString
	err := row.Scan(&dto.ID, &dto.Path, &dto.Slug, &dto.Title, &dto.Kind, &source, &parent, &dto.ChildCount, &dto.EntryCount)
	if err != nil {
		return dto, err
	}
	if source.Valid {
		dto.SourcePath = &source.String
	}
	if parent.Valid {
		dto.ParentPath = &parent.String
	}
	return dto, nil
}

func scanEntries(rows *sql.Rows) ([]entryDTO, error) {
	list := make([]entryDTO, 0)
	for rows.Next() {
		var dto entryDTO
		var nameURL, logoURL, ownerName, ownerURL, websiteURL, description, origin, freePlan, paidPlan, links sql.NullString
		var attrsRaw []byte
		if err := rows.Scan(
			&dto.ID, &dto.Name, &nameURL, &logoURL, &ownerName, &ownerURL,
			&websiteURL, &description, &origin, &freePlan, &paidPlan, &links, &attrsRaw,
		); err != nil {
			return nil, err
		}
		dto.NameURL = nullPtr(nameURL)
		dto.LogoURL = nullPtr(logoURL)
		dto.OwnerName = nullPtr(ownerName)
		dto.OwnerURL = nullPtr(ownerURL)
		dto.WebsiteURL = nullPtr(websiteURL)
		dto.Description = nullPtr(description)
		dto.Origin = nullPtr(origin)
		dto.FreePlan = nullPtr(freePlan)
		dto.PaidPlan = nullPtr(paidPlan)
		dto.Links = nullPtr(links)
		if len(attrsRaw) > 0 && string(attrsRaw) != "{}" && string(attrsRaw) != "null" {
			_ = json.Unmarshal(attrsRaw, &dto.Attrs)
		}
		list = append(list, dto)
	}
	return list, rows.Err()
}

func nullPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}
