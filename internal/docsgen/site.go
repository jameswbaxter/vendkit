package main

import (
	"bytes"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Options controls a single Generate run.
type Options struct {
	DocsDir string // source Markdown directory
	OutDir  string // output site root (accumulates versions)
	Version string // release label for this render, e.g. "v0.2.0"
}

// doc is one source Markdown file discovered under DocsDir.
type doc struct {
	// rel is the POSIX-relative HTML path within a version dir, e.g.
	// "architecture.html" or "specs/cli.html".
	rel     string
	srcPath string // absolute source path
	title   string // first H1, or a derived title
	section string // top-level dir ("" for root, "specs", "design", ...)
}

// Generate renders Options.Version into <OutDir>/<Version>/, merges the version
// into <OutDir>/versions.json, and mirrors the newest version into
// <OutDir>/latest/.
func Generate(opts Options) error {
	if opts.Version == "" {
		return fmt.Errorf("version is required")
	}
	docs, err := discover(opts.DocsDir)
	if err != nil {
		return err
	}
	if len(docs) == 0 {
		return fmt.Errorf("no .md files found under %s", opts.DocsDir)
	}

	// Update the manifest first so pages can bake a current snapshot of the
	// version list (the selector still re-fetches versions.json at runtime).
	manifestPath := filepath.Join(opts.OutDir, manifestName)
	m, err := loadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", manifestName, err)
	}
	m = m.merge(opts.Version)

	versionDir := filepath.Join(opts.OutDir, opts.Version)
	if err := os.RemoveAll(versionDir); err != nil {
		return err
	}
	if err := renderVersion(opts, docs, m, versionDir); err != nil {
		return err
	}

	mb, err := m.marshal()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(manifestPath, mb, 0o644); err != nil {
		return err
	}

	// Mirror the newest version into latest/.
	if m.latest() == opts.Version {
		latestDir := filepath.Join(opts.OutDir, "latest")
		if err := os.RemoveAll(latestDir); err != nil {
			return err
		}
		if err := copyTree(versionDir, latestDir); err != nil {
			return err
		}
	}
	return nil
}

// discover walks DocsDir for *.md files, deriving each doc's output path,
// title, and section. README.md and TEMPLATE.md files are included as regular
// pages. Results are returned in deterministic nav order.
func discover(docsDir string) ([]doc, error) {
	var docs []doc
	err := filepath.WalkDir(docsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(docsDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		htmlRel := strings.TrimSuffix(rel, filepath.Ext(rel)) + ".html"
		section := ""
		if i := strings.IndexByte(rel, '/'); i >= 0 {
			section = rel[:i]
		}
		src, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		docs = append(docs, doc{
			rel:     htmlRel,
			srcPath: p,
			title:   deriveTitle(string(src), rel),
			section: section,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortDocs(docs)
	return docs, nil
}

// sectionOrder gives root docs first, then specs, then design, then anything
// else alphabetically.
func sectionRank(section string) int {
	switch section {
	case "":
		return 0
	case "specs":
		return 1
	case "design":
		return 2
	default:
		return 3
	}
}

func sortDocs(docs []doc) {
	sort.SliceStable(docs, func(i, j int) bool {
		ri, rj := sectionRank(docs[i].section), sectionRank(docs[j].section)
		if ri != rj {
			return ri < rj
		}
		if docs[i].section != docs[j].section {
			return docs[i].section < docs[j].section
		}
		return docs[i].rel < docs[j].rel
	})
}

var h1Re = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)

// deriveTitle returns the first ATX H1, else a title-cased filename.
func deriveTitle(src, rel string) string {
	if m := h1Re.FindStringSubmatch(src); m != nil {
		return strings.TrimSpace(m[1])
	}
	base := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	return base
}

// renderVersion writes every doc plus an index.html into versionDir, and the
// shared stylesheet.
func renderVersion(opts Options, docs []doc, m manifest, versionDir string) error {
	md := newMarkdown()
	if err := os.MkdirAll(filepath.Join(versionDir, "assets"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(versionDir, "assets", "docs.css"), []byte(docsCSS), 0o644); err != nil {
		return err
	}

	for _, d := range docs {
		src, err := os.ReadFile(d.srcPath)
		if err != nil {
			return err
		}
		var body bytes.Buffer
		if err := md.Convert(src, &body); err != nil {
			return fmt.Errorf("render %s: %w", d.srcPath, err)
		}
		page := renderPage(pageData{
			version:  opts.Version,
			rel:      d.rel,
			title:    d.title,
			bodyHTML: body.String(),
			docs:     docs,
			manifest: m,
		})
		outPath := filepath.Join(versionDir, filepath.FromSlash(d.rel))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, []byte(page), 0o644); err != nil {
			return err
		}
	}

	// The version landing page (index.html): a rendered table of contents.
	index := renderPage(pageData{
		version:  opts.Version,
		rel:      "index.html",
		title:    "VendKit docs " + opts.Version,
		bodyHTML: indexBody(opts.Version, docs),
		docs:     docs,
		manifest: m,
	})
	return os.WriteFile(filepath.Join(versionDir, "index.html"), []byte(index), 0o644)
}

// newMarkdown builds a goldmark instance with GFM (tables, strikethrough,
// task lists, autolinks) and an internal-link rewriter that maps *.md → *.html.
func newMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.Prioritized(linkRewriter{}, 100)),
		),
		goldmark.WithRendererOptions(
			gmhtml.WithUnsafe(), // docs are first-party; allows the embedded mermaid blocks etc.
		),
	)
}

// linkRewriter rewrites relative *.md link destinations to *.html so
// doc-to-doc links resolve within the rendered version tree. Fragments and
// query strings are preserved; absolute/external links are left untouched.
type linkRewriter struct{}

func (linkRewriter) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := n.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}
		if dest, changed := rewriteMdLink(string(link.Destination)); changed {
			link.Destination = []byte(dest)
		}
		return ast.WalkContinue, nil
	})
}

// rewriteMdLink maps a relative "<path>.md[#frag][?q]" destination to
// "<path>.html[#frag][?q]". It returns changed=false for external, absolute,
// or non-.md destinations.
func rewriteMdLink(dest string) (string, bool) {
	if dest == "" || strings.Contains(dest, "://") || strings.HasPrefix(dest, "//") ||
		strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, "#") ||
		strings.HasPrefix(dest, "mailto:") {
		return dest, false
	}
	pathPart := dest
	suffix := ""
	if i := strings.IndexAny(dest, "#?"); i >= 0 {
		pathPart, suffix = dest[:i], dest[i:]
	}
	if !strings.HasSuffix(strings.ToLower(pathPart), ".md") {
		return dest, false
	}
	return pathPart[:len(pathPart)-len(".md")] + ".html" + suffix, true
}

// indexBody renders the per-version docs home: a grouped list of every page.
func indexBody(version string, docs []doc) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<h1>VendKit documentation</h1>\n")
	fmt.Fprintf(&b, "<p class=\"vk-lede\">Rendered documentation for release <code>%s</code>. "+
		"Use the version selector above to switch releases.</p>\n", html.EscapeString(version))
	lastSection := "\x00"
	for _, d := range docs {
		if d.section != lastSection {
			if lastSection != "\x00" {
				b.WriteString("</ul>\n")
			}
			label := d.section
			if label == "" {
				label = "Overview"
			}
			fmt.Fprintf(&b, "<h2>%s</h2>\n<ul class=\"vk-index\">\n", html.EscapeString(titleCase(label)))
			lastSection = d.section
		}
		fmt.Fprintf(&b, "  <li><a href=\"%s\">%s</a></li>\n",
			html.EscapeString(d.rel), html.EscapeString(d.title))
	}
	if lastSection != "\x00" {
		b.WriteString("</ul>\n")
	}
	return b.String()
}

// copyTree recursively copies src into dst (files + dirs), preserving layout.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
