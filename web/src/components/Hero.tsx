import type { CSSProperties } from 'react';
import { VersionBadge, hx, ghrepo } from 'go-ui';
import { LIB } from '../data';
import { Html } from './Html';

export interface HeroProps {
  go: (id: string) => void;
}

// Hero is the library masthead: icon, package path, tagline, the live version
// badge and the primary calls to action.
export function Hero({ go }: HeroProps) {
  return (
    <div className="libhero" style={{ '--lib-soft': hx(LIB.accent, '1f'), '--lib-accent': LIB.accent } as CSSProperties}>
      <div className="row">
        <Html html={LIB.icon} className="mono" />
        <div style={{ flex: 1, minWidth: 220 }}>
          <h1 style={{ margin: 0 }}>{LIB.name} <span className="muted" style={{ fontWeight: 400, fontSize: '1rem' }}>for Go</span></h1>
          <div className="pkg mono">{LIB.pkg}</div>
          <p className="tagline">{LIB.tagline}</p>
        </div>
      </div>
      <div className="actions">
        <a className="pill b" href={LIB.repo} target="_blank" rel="noopener"><i className="fa-brands fa-github" />&nbsp;GitHub</a>
        <a className="pill b" href="#docs" onClick={(e) => { e.preventDefault(); go('docs'); }}><i className="fa-solid fa-book" /> API docs</a>
        <VersionBadge repo={ghrepo(LIB)} href={`${LIB.repo}/releases`} />
        <span className="pill">ports <b style={{ color: 'var(--fg)', marginLeft: '.25rem' }}>{LIB.node}</b></span>
      </div>
      <div className="cta" style={{ marginTop: '1.2rem' }}>
        <a className="btn primary" href="#docs" onClick={(e) => { e.preventDefault(); go('docs'); }}>Read the docs →</a>
        <a className="btn" href={LIB.repo} target="_blank" rel="noopener"><i className="fa-brands fa-github" />&nbsp;View on GitHub</a>
      </div>
    </div>
  );
}
