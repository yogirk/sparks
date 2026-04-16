package view

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yogirk/sparks/internal/graph"
	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Options configure the viewer HTTP server.
type Options struct {
	Addr      string // "127.0.0.1:3030" by default; pass "0.0.0.0:3030" only if you know what you're doing
	OpenOnBoot bool   // if true, try to launch the default browser on the listen URL
}

// DefaultAddr is what `sparks view` listens on when no --port flag is given.
// Localhost-only. Exposing the viewer beyond the local machine is a
// deliberate choice, not a default.
const DefaultAddr = "127.0.0.1:3030"

// Server wraps a *http.Server and the vault+manifest it serves.
type Server struct {
	opts   Options
	vault  *vault.Vault
	db     *manifest.DB
	tmpls  *template.Template
	http   *http.Server
}

// NewServer builds a Server bound to the given vault + manifest. The
// manifest is used live — every request re-queries the DB so edits in
// your editor show up on the next page refresh without restarting.
func NewServer(v *vault.Vault, db *manifest.DB, opts Options) (*Server, error) {
	if opts.Addr == "" {
		opts.Addr = DefaultAddr
	}
	tmpls, err := loadTemplates()
	if err != nil {
		return nil, fmt.Errorf("load templates: %w", err)
	}
	return &Server{opts: opts, vault: v, db: db, tmpls: tmpls}, nil
}

// Run starts the HTTP server. Blocks until ctx is canceled or the
// listener errors. A nil ctx is treated as context.Background.
func (s *Server) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	ln, err := net.Listen("tcp", s.opts.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.opts.Addr, err)
	}
	s.http = &http.Server{
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	addr := ln.Addr().String()
	fmt.Fprintf(os.Stdout, "sparks view listening on http://%s (Ctrl-C to stop)\n", addr)
	if s.opts.OpenOnBoot {
		_ = openBrowser("http://" + addr)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- s.http.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /static/", s.handleStatic)
	mux.HandleFunc("GET /tags/{tag}", s.handleTag)

	// Wiki and raw pages share a handler that resolves the path under
	// vault root and renders it.
	mux.HandleFunc("GET /wiki/", s.handlePage)
	mux.HandleFunc("GET /raw/", s.handlePage)
}

// handleIndex renders the landing page: page catalog grouped by type,
// with "Recent" and "Tags" panels in the right sidebar.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	pages, err := s.db.ListPages()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := indexData(s.vault.Config.Vault.Name, pages)
	data.Recent = buildRecent(pages, 10)
	data.Tags = buildTagIndex(s.db, pages)
	if len(data.Tags) > 24 {
		data.Tags = data.Tags[:24]
	}
	s.render(w, "index.html", data)
}

// handleTag renders the page list for a single tag.
func (s *Server) handleTag(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	pages, err := s.db.ListPages()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := tagPageData{
		VaultName: s.vault.Config.Vault.Name,
		Tag:       tag,
		Nav:       buildNav(pages),
		Pages:     buildTagPages(s.db, pages, tag),
		Recent:    buildRecent(pages, 8),
		Tags:      buildTagIndex(s.db, pages),
	}
	if len(data.Tags) > 24 {
		data.Tags = data.Tags[:24]
	}
	s.render(w, "tag.html", data)
}

// handlePage renders a single wiki or raw page. Path is taken from the
// URL (/wiki/entities/Cascade → wiki/entities/Cascade.md).
func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, "/")
	if relPath == "" {
		http.NotFound(w, r)
		return
	}
	// Append .md unless already present. Markdown is the only file
	// format this version renders.
	if !strings.HasSuffix(relPath, ".md") {
		relPath = relPath + ".md"
	}
	full := filepath.Join(s.vault.Root, relPath)
	if !strings.HasPrefix(full, s.vault.Root) {
		http.Error(w, "path escapes vault", http.StatusBadRequest)
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		http.Error(w, fmt.Sprintf("page not found: %s", relPath), http.StatusNotFound)
		return
	}

	resolver, err := s.buildResolver()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fm, htmlStr, _, err := PreparePage(data, resolver)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	title := fm.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(relPath), ".md")
	}

	// Side-panel data. Query once per request — manifest is small and
	// agents editing concurrently should see fresh state.
	pages, _ := s.db.ListPages()
	mPath := filepath.ToSlash(relPath)
	pd := pageData{
		VaultName: s.vault.Config.Vault.Name,
		Path:      mPath,
		Title:     title,
		Body:      template.HTML(htmlStr),
		Nav:       buildNav(pages),
		Backlinks: buildBacklinks(s.db, pages, mPath),
		Meta:      buildPageMeta(s.db, mPath, fm.Type, fm.Maturity, fm.Created, fm.Updated, fm.Aliases, fm.Sources),
	}
	s.render(w, "page.html", pd)
}

// handleStatic serves the embedded CSS.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.StripPrefix("/static/", http.FileServer(http.FS(sub))).ServeHTTP(w, r)
}

// buildResolver constructs a fresh graph.Resolver from the current
// manifest. We rebuild per-request; 262 pages takes microseconds and
// avoids a cache-invalidation story.
func (s *Server) buildResolver() (*graph.Resolver, error) {
	pages, err := s.db.ListPages()
	if err != nil {
		return nil, err
	}
	refs := make([]graph.PageRef, 0, len(pages))
	for _, p := range pages {
		refs = append(refs, graph.PageRef{Path: p.Path, Title: p.Title, Aliases: p.Aliases})
	}
	return graph.NewResolver(refs), nil
}

// render looks up a template by filename and executes it.
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpls.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// loadTemplates parses all *.html files from the embedded templateFS.
func loadTemplates() (*template.Template, error) {
	return template.New("").ParseFS(templateFS, "templates/*.html")
}

// loggingMiddleware emits a one-line log per request. Cheap and useful.
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		fmt.Fprintf(os.Stderr, "%s %s %d %s\n",
			r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond))
	})
}

// --- index page data preparation ---

type indexSection struct {
	Type  string
	Label string
	Pages []indexEntry
}

type indexEntry struct {
	Title string
	Href  string
	Path  string
}

type indexPageData struct {
	VaultName string
	Sections  []indexSection
	Counts    map[string]int
	Recent    []recentPage
	Tags      []tagChip
}

type pageData struct {
	VaultName string
	Path      string
	Title     string
	Body      template.HTML
	Nav       []navSection
	Backlinks []backlink
	Meta      pageMeta
}

// typeOrder is the display order of page types on the index page.
var typeOrder = []string{"entity", "concept", "synthesis", "summary", "collection"}
var typeLabels = map[string]string{
	"entity":     "Entities",
	"concept":    "Concepts",
	"synthesis":  "Synthesis",
	"summary":    "Summaries",
	"collection": "Collections",
}

func indexData(vaultName string, pages []manifest.PageInfo) indexPageData {
	byType := map[string][]indexEntry{}
	counts := map[string]int{}
	for _, p := range pages {
		title := p.Title
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(p.Path), ".md")
		}
		byType[p.Type] = append(byType[p.Type], indexEntry{
			Title: title,
			Href:  pathToHref(p.Path),
			Path:  p.Path,
		})
		counts[p.Type]++
	}
	for t := range byType {
		sort.Slice(byType[t], func(i, j int) bool {
			return strings.ToLower(byType[t][i].Title) < strings.ToLower(byType[t][j].Title)
		})
	}
	sections := make([]indexSection, 0, len(typeOrder))
	for _, t := range typeOrder {
		if list, ok := byType[t]; ok {
			sections = append(sections, indexSection{
				Type:  t,
				Label: typeLabels[t],
				Pages: list,
			})
		}
	}
	return indexPageData{
		VaultName: vaultName,
		Sections:  sections,
		Counts:    counts,
	}
}
