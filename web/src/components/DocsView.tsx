import { DocsApp, VersionBadge, ghrepo } from 'go-ui';
import { LIB } from '../data';
import { SecH } from './SecH';
import { Install } from './Install';
import { QuickStart } from './QuickStart';

// DocsView is the "docs" tab. It renders the full, package-by-package Go API
// reference inline via the shared `DocsApp`, which fetches the generated
// `doc.json` (emitted by docs/gen) and shows a package sidebar + package view,
// hash-routable by import path. A secondary link points at the raw generated
// static HTML (`./api/`). Install + QuickStart snippets follow so a reader can
// get running without leaving the page.
//
// `doc.json` is served at `<base>/doc.json`. If it is missing, DocsApp degrades
// gracefully (it renders an inline error/loading state rather than crashing).
export function DocsView() {
  return (
    <section className="view active" id="view-docs">
      <SecH h="h2">API documentation</SecH>
      <p className="muted">The complete package-by-package Go API reference, generated from source. It documents every exported type, function and method across the {LIB.name} module and its helper packages.</p>

      <div className="cta" style={{ marginBottom: '1.4rem' }}>
        <a className="btn" href="./api/" target="_blank" rel="noopener"><i className="fa-solid fa-file-code" />&nbsp;Raw generated HTML</a>
        <a className="btn" href={LIB.repo} target="_blank" rel="noopener"><i className="fa-brands fa-github" />&nbsp;Source on GitHub</a>
        <VersionBadge repo={ghrepo(LIB)} href={`${LIB.repo}/releases`} />
      </div>

      <DocsApp url={`${import.meta.env.BASE_URL}doc.json`} />

      <Install />
      <QuickStart />
    </section>
  );
}
