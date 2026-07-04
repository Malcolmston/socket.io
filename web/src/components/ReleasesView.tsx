import { ReleaseList, ghrepo } from 'go-ui';
import type { RelLib } from 'go-ui';
import { LIB } from '../data';
import { SecH } from './SecH';

// Live release history for this single repository, read from the GitHub
// Releases API.
const RELEASE_LIBS: RelLib[] = [
  { name: LIB.name, icon: LIB.icon, accent: LIB.accent, repo: ghrepo(LIB), url: LIB.repo },
];

// ReleasesView renders the live release-history + changelog tab.
export function ReleasesView() {
  return (
    <section className="view active" id="view-releases">
      <SecH h="h2">Releases &amp; changelog</SecH>
      <p className="muted">Socket.IO for Go ships automated semver releases — the moment a <code>VERSION</code> bump lands on <code>main</code>, a tag and GitHub Release are cut and the moving <code>stable</code> tag advances. The list below is read <b>live</b> from the GitHub Releases API, newest first, so it is never out of date. Full history lives in the repository's <code>CHANGELOG.md</code>.</p>
      <div style={{ marginTop: '1.4rem' }}><ReleaseList libs={RELEASE_LIBS} /></div>
    </section>
  );
}
