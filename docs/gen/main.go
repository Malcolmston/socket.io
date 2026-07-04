// Command gendocs generates a static HTML documentation site for a Go module
// using only the standard library (go/doc, go/parser). It walks every package
// in the module, renders package docs, exported functions, types, and methods,
// and writes a browsable site to an output directory (default ./_site).
//
// Usage:
//
//	go run ./docs/gen -out _site
//
// It is intentionally dependency-free so it runs in CI without any third-party
// tooling, matching this project's standard-library-only ethos.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"html"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	out := flag.String("out", "_site", "output directory")
	title := flag.String("title", "", "site title (defaults to module path)")
	flag.Parse()

	modPath, err := modulePath("go.mod")
	if err != nil {
		fatal(err)
	}
	if *title == "" {
		*title = modPath
	}

	pkgs, err := collectPackages(".", modPath)
	if err != nil {
		fatal(err)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ImportPath < pkgs[j].ImportPath })

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fatal(err)
	}
	if err := writeIndex(*out, *title, modPath, pkgs); err != nil {
		fatal(err)
	}
	for _, p := range pkgs {
		if err := writePackage(*out, *title, modPath, p); err != nil {
			fatal(err)
		}
	}
	// A .nojekyll file stops GitHub Pages from running the content through
	// Jekyll (which would drop files/dirs beginning with underscores).
	_ = os.WriteFile(filepath.Join(*out, ".nojekyll"), nil, 0o644)
	fmt.Printf("gendocs: wrote %d package pages to %s\n", len(pkgs), *out)
}

// pkgInfo is a documented package.
type pkgInfo struct {
	ImportPath string
	Rel        string // path relative to module root ("" for root)
	Doc        *doc.Package
	Fset       *token.FileSet
}

// Synopsis returns the first sentence of the package doc.
func (p pkgInfo) Synopsis() string {
	if p.Doc == nil {
		return ""
	}
	return doc.Synopsis(p.Doc.Doc)
}

// Name returns the package's Go name.
func (p pkgInfo) Name() string {
	if p.Doc != nil {
		return p.Doc.Name
	}
	return filepath.Base(p.ImportPath)
}

func collectPackages(root, modPath string) ([]pkgInfo, error) {
	var pkgs []pkgInfo
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		base := info.Name()
		if path != root && (base == "testdata" || base == "_site" || base == ".git" ||
			strings.HasPrefix(base, ".") || base == "node_modules") {
			return filepath.SkipDir
		}
		p, ok := parsePackage(path, root, modPath)
		if ok {
			pkgs = append(pkgs, p)
		}
		return nil
	})
	return pkgs, err
}

func parsePackage(dir, root, modPath string) (pkgInfo, bool) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil || len(parsed) == 0 {
		return pkgInfo{}, false
	}
	// Choose the non-main, non-test package (skip package main commands only if
	// they carry no doc; we still document them).
	var astPkg *ast.Package
	for name, ap := range parsed {
		if strings.HasSuffix(name, "_test") {
			continue
		}
		astPkg = ap
		break
	}
	if astPkg == nil {
		return pkgInfo{}, false
	}

	rel, _ := filepath.Rel(root, dir)
	if rel == "." {
		rel = ""
	}
	importPath := modPath
	if rel != "" {
		importPath = modPath + "/" + filepath.ToSlash(rel)
	}
	// Mode 0 keeps only exported symbols — this is a public API reference.
	dpkg := doc.New(astPkg, importPath, 0)
	return pkgInfo{ImportPath: importPath, Rel: rel, Doc: dpkg, Fset: fset}, true
}

func modulePath(goMod string) (string, error) {
	f, err := os.Open(goMod)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}
	return "", fmt.Errorf("gendocs: no module directive in %s", goMod)
}

// ---- rendering --------------------------------------------------------------

const style = `
:root{
  --bg:#06070a; --glass:rgba(255,255,255,.055); --glass-2:rgba(255,255,255,.09);
  --edge:rgba(255,255,255,.10); --edge-2:rgba(255,255,255,.18); --code-bg:rgba(10,12,18,.72);
  --fg:#f5f6f8; --fg-muted:#a6adba; --fg-dim:#727a88;
  --link:#7db3ff; --grad:linear-gradient(120deg,#6ea8ff,#a58bff);
  --radius-sm:14px; --blur:saturate(180%) blur(30px); --hi:inset 0 1px 0 rgba(255,255,255,.16);
}
@media (prefers-color-scheme: light){
  :root{
    --bg:#f5f5f7; --glass:rgba(255,255,255,.6); --glass-2:rgba(255,255,255,.75);
    --edge:rgba(255,255,255,.7); --edge-2:rgba(0,0,0,.08); --code-bg:rgba(255,255,255,.72);
    --fg:#1d1d1f; --fg-muted:#4b4f57; --fg-dim:#86868b; --link:#0066cc;
    --hi:inset 0 1px 0 rgba(255,255,255,.9);
  }
}
*{box-sizing:border-box}
html{scroll-behavior:smooth}
body{margin:0;background:var(--bg);color:var(--fg);
  font:16px/1.65 -apple-system,BlinkMacSystemFont,"SF Pro Text","Inter","Segoe UI",Roboto,Helvetica,Arial,sans-serif;
  -webkit-font-smoothing:antialiased;letter-spacing:-.011em;overflow-x:hidden}
body::before{content:"";position:fixed;inset:0;z-index:-2;background:var(--bg)}
body::after{content:"";position:fixed;inset:-30%;z-index:-1;pointer-events:none;filter:blur(100px);opacity:.55;
  background:
   radial-gradient(24% 28% at 22% 26%,rgba(72,116,196,.5),transparent 62%),
   radial-gradient(22% 26% at 78% 18%,rgba(126,102,200,.46),transparent 62%),
   radial-gradient(28% 30% at 62% 84%,rgba(46,124,148,.38),transparent 62%);
  animation:flow 30s ease-in-out infinite alternate}
@keyframes flow{0%{transform:translate3d(0,0,0) scale(1)}50%{transform:translate3d(-3%,3%,0) scale(1.1)}100%{transform:translate3d(3%,-2%,0) scale(1.05)}}
@media (prefers-reduced-motion:reduce){body::after{animation:none}}
@media (prefers-color-scheme: light){body::after{opacity:.4}}
a{color:var(--link);text-decoration:none}
a:hover{opacity:.82}
code,pre{font-family:"SF Mono",ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:.88em}
.wrap{max-width:920px;margin:0 auto;padding:0 1.5rem}
header.nav{position:sticky;top:0;z-index:20;background:color-mix(in srgb,var(--bg) 62%,transparent);
  backdrop-filter:var(--blur);-webkit-backdrop-filter:var(--blur);border-bottom:1px solid var(--edge)}
.nav-inner{max-width:920px;margin:0 auto;display:flex;align-items:center;gap:.8rem;padding:.6rem 1.5rem;flex-wrap:wrap}
.brand{display:flex;align-items:center;gap:.5rem;font-weight:600;color:var(--fg);letter-spacing:-.02em}
.brand:hover{opacity:1}
.brand .logo{width:26px;height:26px;border-radius:8px;background:var(--grad);display:grid;place-items:center;
  color:#03121b;font-weight:800;font-size:.72rem;box-shadow:0 6px 16px -6px rgba(110,168,255,.7)}
.brand .sub{color:var(--fg-dim);font-weight:500}
.nav-ctx{margin-left:auto;color:var(--fg-dim);font-size:.85rem;font-family:"SF Mono",monospace;word-break:break-all}
main.wrap{padding-top:2.4rem;padding-bottom:6rem}
h1{font-size:2rem;letter-spacing:-.03em;margin:.35rem 0 .3rem;font-weight:600;overflow-wrap:anywhere}
h1 .cmd{font-size:.68rem;font-weight:600;vertical-align:middle;margin-left:.5rem;padding:.15rem .55rem;border-radius:999px;
  background:var(--glass-2);border:1px solid var(--edge);color:var(--fg-muted)}
h2{font-size:1.3rem;letter-spacing:-.02em;margin:2.6rem 0 .8rem;display:flex;align-items:center;gap:.6rem;font-weight:600}
h2::before{content:"";width:4px;height:1.05em;border-radius:3px;background:var(--grad)}
h3{font-size:1.02rem;margin:1.6rem 0 .4rem;font-family:"SF Mono",monospace;font-weight:600;color:var(--fg)}
.crumb{color:var(--fg-dim);font-size:.9rem;margin:.2rem 0}
.import{display:inline-block;margin:.5rem 0 0;padding:.4rem .85rem;border-radius:999px;background:var(--glass);
  backdrop-filter:var(--blur);border:1px solid var(--edge);box-shadow:var(--hi);font-size:.85rem;color:var(--fg-muted);word-break:break-all}
.doc{white-space:pre-wrap;margin:.5rem 0 1.2rem;color:var(--fg-muted)}
pre{background:var(--code-bg);backdrop-filter:var(--blur);border:1px solid var(--edge);border-radius:var(--radius-sm);
  padding:.9rem 1.1rem;overflow:auto;box-shadow:var(--hi);color:var(--fg);line-height:1.6}
:not(pre)>code{background:var(--glass-2);border:1px solid var(--edge);border-radius:6px;padding:.06em .4em;color:var(--fg)}
.count{color:var(--fg-muted);margin:.4rem 0 1.4rem}
.pkgs{list-style:none;padding:0;margin:0;display:grid;gap:.7rem}
.pkgs a.card{display:block;padding:1rem 1.2rem;border-radius:var(--radius-sm);background:var(--glass);
  backdrop-filter:var(--blur);border:1px solid var(--edge);box-shadow:var(--hi);
  transition:transform .2s,border-color .2s,background .2s}
.pkgs a.card:hover{transform:translateY(-3px);border-color:var(--edge-2);background:var(--glass-2);opacity:1}
.pkgs .name{font-family:"SF Mono",monospace;font-weight:600;color:var(--fg);word-break:break-all}
.pkgs .syn{color:var(--fg-muted);display:block;font-size:.9rem;margin-top:.25rem}
.badge{display:inline-block;background:var(--glass-2);border:1px solid var(--edge);color:var(--fg-muted);border-radius:999px;
  padding:.05rem .55rem;font-size:.72rem;font-family:"SF Mono",monospace;vertical-align:middle;margin-left:.5rem}
footer{color:var(--fg-dim);font-size:.85rem;margin-top:3.5rem;padding-top:1.5rem;border-top:1px solid var(--edge);text-align:center}
footer a{color:var(--fg-muted)}
`

func page(title, body string) string {
	return "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\">" +
		"<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">" +
		"<title>" + html.EscapeString(title) + "</title><style>" + style + "</style></head><body>" +
		body + "</body></html>"
}

func writeIndex(out, title, modPath string, pkgs []pkgInfo) error {
	var b strings.Builder
	b.WriteString(navbar(modPath))
	b.WriteString(`<main class="wrap">`)
	b.WriteString(`<div class="crumb">Package documentation</div>`)
	b.WriteString(`<h1>` + html.EscapeString(title) + `</h1>`)
	b.WriteString(`<p class="count">` + fmt.Sprintf("%d packages.", len(pkgs)) + `</p>`)
	b.WriteString(`<ul class="pkgs">`)
	for _, p := range pkgs {
		label := p.ImportPath
		b.WriteString(`<li><a class="card" href="` + pkgHref(p) + `"><span class="name">` + html.EscapeString(label) + `</span>`)
		if syn := p.Synopsis(); syn != "" {
			b.WriteString(`<span class="syn">` + html.EscapeString(syn) + `</span>`)
		}
		b.WriteString(`</a></li>`)
	}
	b.WriteString(`</ul>`)
	b.WriteString(footer())
	b.WriteString(`</main>`)
	return os.WriteFile(filepath.Join(out, "index.html"), []byte(page(title, b.String())), 0o644)
}

func pkgHref(p pkgInfo) string {
	if p.Rel == "" {
		return "./pkg/index.html"
	}
	return "./pkg/" + filepath.ToSlash(p.Rel) + "/index.html"
}

func writePackage(out, title, modPath string, p pkgInfo) error {
	dir := filepath.Join(out, "pkg")
	if p.Rel != "" {
		dir = filepath.Join(dir, filepath.FromSlash(p.Rel))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// depth from this page back to site root, for relative links.
	up := relUp(p)

	var b strings.Builder
	b.WriteString(navbar(modPath))
	b.WriteString(`<main class="wrap">`)
	b.WriteString(`<div class="crumb"><a href="` + up + `index.html">‹ ` + html.EscapeString(title) + `</a></div>`)
	b.WriteString(`<h1>package ` + html.EscapeString(p.Name()))
	if p.Doc != nil && p.Doc.Name == "main" {
		b.WriteString(`<span class="cmd">command</span>`)
	}
	b.WriteString(`</h1>`)
	b.WriteString(`<div><span class="import">import "` + html.EscapeString(p.ImportPath) + `"</span></div>`)

	if p.Doc != nil && strings.TrimSpace(p.Doc.Doc) != "" {
		b.WriteString(`<div class="doc">` + html.EscapeString(p.Doc.Doc) + `</div>`)
	}

	if p.Doc != nil {
		if len(p.Doc.Consts) > 0 || len(p.Doc.Vars) > 0 {
			b.WriteString(`<h2>Constants &amp; Variables</h2>`)
			for _, c := range p.Doc.Consts {
				writeValue(&b, p, c)
			}
			for _, v := range p.Doc.Vars {
				writeValue(&b, p, v)
			}
		}
		if len(p.Doc.Funcs) > 0 {
			b.WriteString(`<h2>Functions</h2>`)
			for _, fn := range p.Doc.Funcs {
				writeFunc(&b, p, fn)
			}
		}
		if len(p.Doc.Types) > 0 {
			b.WriteString(`<h2>Types</h2>`)
			for _, t := range p.Doc.Types {
				writeType(&b, p, t)
			}
		}
	}

	b.WriteString(footer())
	b.WriteString(`</main>`)
	return os.WriteFile(filepath.Join(dir, "index.html"), []byte(page("package "+p.Name()+" · "+title, b.String())), 0o644)
}

func writeValue(b *strings.Builder, p pkgInfo, v *doc.Value) {
	b.WriteString(`<pre>` + html.EscapeString(nodeString(p.Fset, v.Decl)) + `</pre>`)
	if strings.TrimSpace(v.Doc) != "" {
		b.WriteString(`<div class="doc">` + html.EscapeString(v.Doc) + `</div>`)
	}
}

func writeFunc(b *strings.Builder, p pkgInfo, fn *doc.Func) {
	b.WriteString(`<h3>func ` + html.EscapeString(fn.Name) + `</h3>`)
	b.WriteString(`<pre>` + html.EscapeString(oneLine(nodeString(p.Fset, fn.Decl))) + `</pre>`)
	if strings.TrimSpace(fn.Doc) != "" {
		b.WriteString(`<div class="doc">` + html.EscapeString(fn.Doc) + `</div>`)
	}
}

func writeType(b *strings.Builder, p pkgInfo, t *doc.Type) {
	b.WriteString(`<h3>type ` + html.EscapeString(t.Name) + `</h3>`)
	b.WriteString(`<pre>` + html.EscapeString(nodeString(p.Fset, t.Decl)) + `</pre>`)
	if strings.TrimSpace(t.Doc) != "" {
		b.WriteString(`<div class="doc">` + html.EscapeString(t.Doc) + `</div>`)
	}
	for _, fn := range t.Funcs {
		writeFunc(b, p, fn)
	}
	for _, m := range t.Methods {
		writeFunc(b, p, m)
	}
}

func relUp(p pkgInfo) string {
	depth := 1 // pkg/
	if p.Rel != "" {
		depth += strings.Count(filepath.ToSlash(p.Rel), "/") + 1
	}
	return strings.Repeat("../", depth)
}

func footer() string {
	return `<footer>Part of <a href="https://malcolmston.github.io/go/">malcolmston/go</a> · generated by gendocs · standard library only</footer>`
}

// navbar renders the shared Liquid-Glass top bar linking back to the unified site.
func navbar(modPath string) string {
	return `<header class="nav"><div class="nav-inner">` +
		`<a class="brand" href="https://malcolmston.github.io/go/"><span class="logo">go</span>malcolmston<span class="sub">/go</span></a>` +
		`<span class="nav-ctx">` + html.EscapeString(modPath) + `</span>` +
		`</div></header>`
}

// nodeString renders an AST node back to source text.
func nodeString(fset *token.FileSet, node any) string {
	var b strings.Builder
	if err := printerFprint(&b, fset, node); err != nil {
		return fmt.Sprintf("%v", node)
	}
	return b.String()
}

func oneLine(s string) string {
	// Collapse a multi-line signature into a compact single line for readability.
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// printerFprint writes an AST node as gofmt-style source to w.
func printerFprint(w io.Writer, fset *token.FileSet, node any) error {
	cfg := &printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}
	return cfg.Fprint(w, fset, node)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gendocs:", err)
	os.Exit(1)
}
