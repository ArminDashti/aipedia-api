package bookmarks

import (
	"regexp"
	"strings"
)

var (
	reHeading    = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	reImgSrc     = regexp.MustCompile(`(?i)<img[^>]+src=["']([^"']+)["']`)
	reMDLink     = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	reHTMLTag    = regexp.MustCompile(`(?s)<[^>]+>`)
	reMultiSpace = regexp.MustCompile(`\s+`)
)

// ParsedTable is one markdown catalog file.
type ParsedTable struct {
	Title   string
	Headers []string
	Rows    []map[string]string
}

// ParseMarkdownFile extracts the first GFM table and H1 title.
func ParseMarkdownFile(content string) (ParsedTable, error) {
	out := ParsedTable{Title: "", Headers: nil, Rows: nil}
	if m := reHeading.FindStringSubmatch(content); len(m) == 2 {
		out.Title = strings.TrimSpace(m[1])
	}

	lines := strings.Split(content, "\n")
	var tableLines []string
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") {
			inTable = true
			tableLines = append(tableLines, trimmed)
			continue
		}
		if inTable {
			break
		}
	}
	if len(tableLines) < 2 {
		return out, nil
	}

	headers := splitRow(tableLines[0])
	start := 1
	if len(tableLines) > 1 && isSeparatorRow(tableLines[1]) {
		start = 2
	}
	out.Headers = headers
	for i := start; i < len(tableLines); i++ {
		cells := splitRow(tableLines[i])
		if len(cells) == 0 {
			continue
		}
		row := map[string]string{}
		for hi, h := range headers {
			key := normalizeHeader(h)
			val := ""
			if hi < len(cells) {
				val = strings.TrimSpace(cells[hi])
			}
			row[key] = val
		}
		if rowEmpty(row) {
			continue
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func isSeparatorRow(line string) bool {
	cells := splitRow(line)
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		t := strings.ReplaceAll(c, ":", "")
		t = strings.ReplaceAll(t, "-", "")
		t = strings.TrimSpace(t)
		if t != "" {
			return false
		}
	}
	return true
}

func normalizeHeader(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	h = strings.ReplaceAll(h, " ", "_")
	return h
}

func rowEmpty(row map[string]string) bool {
	for _, v := range row {
		if strings.TrimSpace(stripTags(v)) != "" {
			return false
		}
	}
	return true
}

// EntryFields are normalized columns for DB insert.
type EntryFields struct {
	Name        string
	NameURL     string
	LogoURL     string
	OwnerName   string
	OwnerURL    string
	WebsiteURL  string
	Description string
	Origin      string
	FreePlan    string
	PaidPlan    string
	Links       string
	Attrs       map[string]string
}

// MapRow converts a raw table row into EntryFields.
func MapRow(row map[string]string) EntryFields {
	known := map[string]bool{
		"logo": true, "app": true, "owner": true, "origin": true,
		"free_plan": true, "paid_plan": true, "free_credits": true, "price": true,
		"description": true, "links": true, "website": true,
	}
	ef := EntryFields{Attrs: map[string]string{}}

	if v, ok := row["logo"]; ok {
		ef.LogoURL = firstImgSrc(v)
	}
	if v, ok := row["app"]; ok {
		ef.Name, ef.NameURL = firstLink(v)
		if ef.Name == "" {
			ef.Name = plainText(v)
		}
	}
	if v, ok := row["owner"]; ok {
		ef.OwnerName, ef.OwnerURL = firstLink(v)
		if ef.OwnerName == "" {
			ef.OwnerName = plainText(v)
		}
	}
	if v, ok := row["origin"]; ok {
		ef.Origin = plainText(v)
	}
	if v, ok := row["free_plan"]; ok {
		ef.FreePlan = plainText(v)
	}
	if v, ok := row["paid_plan"]; ok {
		ef.PaidPlan = plainText(v)
	}
	if v, ok := row["free_credits"]; ok {
		ef.FreePlan = plainText(v)
	}
	if v, ok := row["price"]; ok {
		ef.PaidPlan = plainText(v)
	}
	if v, ok := row["description"]; ok {
		ef.Description = plainText(v)
	}
	if v, ok := row["links"]; ok {
		ef.Links = plainText(v)
	}
	if v, ok := row["website"]; ok {
		_, url := firstLink(v)
		if url != "" {
			ef.WebsiteURL = url
		} else {
			ef.WebsiteURL = plainText(v)
		}
	}
	if ef.WebsiteURL == "" && ef.NameURL != "" {
		ef.WebsiteURL = ef.NameURL
	}
	for k, v := range row {
		if known[k] {
			continue
		}
		if strings.TrimSpace(v) != "" {
			ef.Attrs[k] = plainText(v)
		}
	}
	return ef
}

func firstImgSrc(s string) string {
	if m := reImgSrc.FindStringSubmatch(s); len(m) == 2 {
		return m[1]
	}
	return ""
}

func firstLink(s string) (text, url string) {
	if m := reMDLink.FindStringSubmatch(s); len(m) == 3 {
		return strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
	}
	return "", ""
}

func stripTags(s string) string {
	return reHTMLTag.ReplaceAllString(s, " ")
}

func plainText(s string) string {
	s = reMDLink.ReplaceAllString(s, "$1")
	s = stripTags(s)
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// TitleFromSlug builds a display title from a path segment.
func TitleFromSlug(slug string) string {
	switch strings.ToLower(slug) {
	case "ai":
		return "AI"
	case "mcps":
		return "MCPs"
	case "cli":
		return "CLI"
	case "dns":
		return "DNS"
	case "ui":
		return "UI"
	case "rag":
		return "RAG"
	}
	slug = strings.ReplaceAll(slug, "-", " ")
	slug = strings.ReplaceAll(slug, "_", " ")
	parts := strings.Fields(slug)
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
