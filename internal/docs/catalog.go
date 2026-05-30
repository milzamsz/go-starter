package docs

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Page struct {
	Title       string
	Summary     string
	Path        string
	Section     string
	ContentPath string
}

type Link struct {
	Title string
	Path  string
}

type NavItem struct {
	Title  string
	Summary string
	Path   string
	Active bool
}

type NavGroup struct {
	Title string
	Items []NavItem
}

type TOCItem struct {
	Title string
	ID    string
	Level int
}

type Article struct {
	Page
	ContentHTML template.HTML
	TOC         []TOCItem
	Prev        *Link
	Next        *Link
	Nav         []NavGroup
}

var pages = []Page{
	{Title: "Introduction", Summary: "What GoStarter includes and who it is for.", Path: "/docs/introduction", Section: "Introduction", ContentPath: "content/docs/introduction.md"},
	{Title: "Installation", Summary: "Run GoStarter locally in minutes.", Path: "/docs/installation", Section: "Introduction", ContentPath: "content/docs/installation.md"},
	{Title: "Configuration", Summary: "Environment variables and runtime settings.", Path: "/docs/configuration", Section: "Introduction", ContentPath: "content/docs/configuration.md"},
	{Title: "Architecture", Summary: "How the codebase is structured.", Path: "/docs/architecture", Section: "Introduction", ContentPath: "content/docs/architecture.md"},
	{Title: "Philosophy", Summary: "Design principles behind GoStarter.", Path: "/docs/philosophy", Section: "Introduction", ContentPath: "content/docs/philosophy.md"},
	{Title: "Deployment", Summary: "Ship with Docker and production env settings.", Path: "/docs/guides/deployment", Section: "Guides", ContentPath: "content/docs/guides/deployment.md"},
	{Title: "PostgreSQL Setup", Summary: "Database setup, migration, and seeding.", Path: "/docs/guides/postgresql-setup", Section: "Guides", ContentPath: "content/docs/guides/postgresql-setup.md"},
	{Title: "Local Development", Summary: "Daily workflow for app, worker, and CSS.", Path: "/docs/guides/local-development", Section: "Guides", ContentPath: "content/docs/guides/local-development.md"},
	{Title: "Updating GoStarter", Summary: "How to safely update and validate changes.", Path: "/docs/guides/updating-gostarter", Section: "Guides", ContentPath: "content/docs/guides/updating-gostarter.md"},
	{Title: "Authentication", Summary: "JWT, web sessions, OAuth, and 2FA.", Path: "/docs/features/authentication", Section: "Features", ContentPath: "content/docs/features/authentication.md"},
	{Title: "Billing", Summary: "Stripe Checkout, Portal, and webhooks.", Path: "/docs/features/billing", Section: "Features", ContentPath: "content/docs/features/billing.md"},
	{Title: "Email", Summary: "SMTP sender and queued email tasks.", Path: "/docs/features/email", Section: "Features", ContentPath: "content/docs/features/email.md"},
	{Title: "Security", Summary: "CSRF, CORS, CSP, session cookies, and RBAC.", Path: "/docs/features/security", Section: "Features", ContentPath: "content/docs/features/security.md"},
	{Title: "Storage", Summary: "Local and object storage behavior.", Path: "/docs/features/storage", Section: "Features", ContentPath: "content/docs/features/storage.md"},
	{Title: "Background Jobs", Summary: "Asynq queues, workers, and retry behavior.", Path: "/docs/features/background-jobs", Section: "Features", ContentPath: "content/docs/features/background-jobs.md"},
	{Title: "Components", Summary: "Templ UI primitives and usage patterns.", Path: "/docs/features/components", Section: "Features", ContentPath: "content/docs/features/components.md"},
}

func DefaultPath() string {
	return pages[0].Path
}

func LandingGroups() []NavGroup {
	return buildNav("")
}

func BuildArticle(path string) (*Article, bool, error) {
	index := -1
	for i := range pages {
		if pages[i].Path == path {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, false, nil
	}

	raw, err := os.ReadFile(filepath.Clean(pages[index].ContentPath))
	if err != nil {
		return nil, true, err
	}
	contentHTML, toc, err := renderMarkdown(raw)
	if err != nil {
		return nil, true, err
	}

	article := &Article{
		Page:        pages[index],
		ContentHTML: contentHTML,
		TOC:         toc,
		Nav:         buildNav(path),
	}
	if index > 0 {
		article.Prev = &Link{Title: pages[index-1].Title, Path: pages[index-1].Path}
	}
	if index < len(pages)-1 {
		article.Next = &Link{Title: pages[index+1].Title, Path: pages[index+1].Path}
	}
	return article, true, nil
}

func buildNav(activePath string) []NavGroup {
	order := []string{"Introduction", "Guides", "Features"}
	grouped := make(map[string][]NavItem, len(order))
	for _, p := range pages {
		grouped[p.Section] = append(grouped[p.Section], NavItem{
			Title:  p.Title,
			Summary: p.Summary,
			Path:   p.Path,
			Active: p.Path == activePath,
		})
	}
	nav := make([]NavGroup, 0, len(order))
	for _, section := range order {
		nav = append(nav, NavGroup{Title: section, Items: grouped[section]})
	}
	return nav
}

func renderMarkdown(raw []byte) (template.HTML, []TOCItem, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)

	reader := text.NewReader(raw)
	doc := md.Parser().Parse(reader)
	stripLeadingH1(doc)
	toc := buildTOC(doc, raw)

	var out bytes.Buffer
	if err := md.Renderer().Render(&out, raw, doc); err != nil {
		return "", nil, err
	}
	return template.HTML(out.String()), toc, nil
}

func stripLeadingH1(doc ast.Node) {
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		heading, ok := child.(*ast.Heading)
		if !ok {
			continue
		}
		if heading.Level == 1 {
			doc.RemoveChild(doc, heading)
		}
		return
	}
}

func buildTOC(doc ast.Node, raw []byte) []TOCItem {
	items := make([]TOCItem, 0, 8)
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := n.(*ast.Heading)
		if !ok || (heading.Level != 2 && heading.Level != 3) {
			return ast.WalkContinue, nil
		}
		id := ""
		if v, ok := heading.AttributeString("id"); ok {
			if b, ok := v.([]byte); ok {
				id = string(b)
			}
		}
		if id == "" {
			id = slugify(string(heading.Text(raw)))
		}
		items = append(items, TOCItem{
			Title: string(heading.Text(raw)),
			ID:    id,
			Level: heading.Level,
		})
		return ast.WalkContinue, nil
	})
	return items
}

func slugify(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	s = strings.ReplaceAll(s, "`", "")
	replacer := strings.NewReplacer(
		" ", "-", "/", "-", "\\", "-", "_", "-", ".", "-", ",", "",
		":", "", ";", "", "(", "", ")", "", "[", "", "]", "",
	)
	s = replacer.Replace(s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
