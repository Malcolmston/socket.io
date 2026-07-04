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
:root{color-scheme:light dark}
*{box-sizing:border-box}
body{margin:0;font:16px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;
 color:#1a1a1a;background:#fff}
.wrap{max-width:900px;margin:0 auto;padding:2rem 1.25rem 5rem}
header{border-bottom:1px solid #e5e5e5;background:#fafafa}
header .wrap{padding:1.5rem 1.25rem}
h1{font-size:1.6rem;margin:.2rem 0}
h1 a{color:inherit;text-decoration:none}
h2{font-size:1.3rem;margin:2.2rem 0 .6rem;padding-top:.4rem;border-top:1px solid #eee}
h3{font-size:1.05rem;margin:1.6rem 0 .3rem;font-family:ui-monospace,SFMono-Regular,Menlo,monospace}
a{color:#0b6bcb;text-decoration:none}
a:hover{text-decoration:underline}
code,pre{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:.9em}
pre{background:#f6f8fa;border:1px solid #eaecef;border-radius:6px;padding:.85rem 1rem;overflow:auto}
.doc{white-space:pre-wrap;margin:.3rem 0 1rem}
.pkgs{list-style:none;padding:0;margin:1.5rem 0}
.pkgs li{padding:.55rem 0;border-bottom:1px solid #f0f0f0}
.pkgs .name{font-family:ui-monospace,monospace;font-weight:600}
.pkgs .syn{color:#555;display:block;font-size:.92rem}
.crumb{color:#888;font-size:.9rem;margin-bottom:.3rem}
.badge{display:inline-block;background:#eef4ff;color:#0b6bcb;border-radius:4px;padding:0 .4rem;font-size:.75rem;
 font-family:ui-monospace,monospace;vertical-align:middle;margin-left:.4rem}
footer{color:#999;font-size:.85rem;margin-top:3rem;text-align:center}
@media (prefers-color-scheme:dark){
 body{color:#e6e6e6;background:#0d1117}
 header{background:#161b22;border-color:#30363d}
 h2{border-color:#21262d}h3{}
 pre{background:#161b22;border-color:#30363d}
 .pkgs li{border-color:#21262d}.pkgs .syn{color:#9aa4af}
 .badge{background:#1c2f4a;color:#66b2ff}
 a{color:#66b2ff}.crumb{color:#8b949e}
}
`

func page(title, body string) string {
	return "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\">" +
		"<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">" +
		"<title>" + html.EscapeString(title) + "</title><style>" + style + "</style></head><body>" +
		body + "</body></html>"
}

func writeIndex(out, title, modPath string, pkgs []pkgInfo) error {
	var b strings.Builder
	b.WriteString(`<header><div class="wrap"><h1>` + html.EscapeString(title) + `</h1>`)
	b.WriteString(`<div class="crumb">Package documentation &middot; ` + html.EscapeString(modPath) + `</div></div></header>`)
	b.WriteString(`<main class="wrap">`)
	b.WriteString(`<p>` + fmt.Sprintf("%d packages.", len(pkgs)) + `</p>`)
	b.WriteString(`<ul class="pkgs">`)
	for _, p := range pkgs {
		label := p.ImportPath
		b.WriteString(`<li><a href="` + pkgHref(p) + `"><span class="name">` + html.EscapeString(label) + `</span></a>`)
		if syn := p.Synopsis(); syn != "" {
			b.WriteString(`<span class="syn">` + html.EscapeString(syn) + `</span>`)
		}
		b.WriteString(`</li>`)
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
	b.WriteString(`<header><div class="wrap">`)
	b.WriteString(`<div class="crumb"><a href="` + up + `index.html">` + html.EscapeString(title) + `</a></div>`)
	b.WriteString(`<h1>package ` + html.EscapeString(p.Name()))
	if p.Doc != nil && p.Doc.Name == "main" {
		b.WriteString(`<span class="badge">command</span>`)
	}
	b.WriteString(`</h1>`)
	b.WriteString(`<div class="crumb"><code>import "` + html.EscapeString(p.ImportPath) + `"</code></div>`)
	b.WriteString(`</div></header><main class="wrap">`)

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
	return `<footer>Generated by gendocs · standard library only</footer>`
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
