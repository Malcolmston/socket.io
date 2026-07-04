import { LIB } from '../data';
import { SecH } from './SecH';

// DocsView links to the generated Go API reference. The reference is produced
// by the dependency-free `docs/gen` tool at build time and published under
// `./api/` next to this landing site (see .github/workflows/pages.yml).
export function DocsView() {
  return (
    <section className="view active" id="view-docs">
      <SecH h="h2">API reference</SecH>
      <p className="muted">The complete, generated Go API reference — every package, exported function, type and method — is published alongside this site. It is produced by the standard-library-only generator committed in the repository under <code>docs/gen</code>.</p>
      <div className="cta" style={{ marginTop: '1.2rem' }}>
        <a className="btn primary" href="./api/" target="_blank" rel="noopener"><i className="fa-solid fa-book" />&nbsp;Open the API reference →</a>
        <a className="btn" href={LIB.repo} target="_blank" rel="noopener"><i className="fa-brands fa-github" />&nbsp;Source on GitHub</a>
      </div>
      <div className="note" style={{ marginTop: '1.4rem' }}>Prefer the command line? <code>go doc {LIB.pkg}</code> renders the same documentation in your terminal.</div>
    </section>
  );
}
