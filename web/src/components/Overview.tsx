import { LIB } from '../data';
import { Hero } from './Hero';
import { Install } from './Install';
import { QuickStart } from './QuickStart';
import { NodeVsGo } from './NodeVsGo';
import { Features } from './Features';

export interface OverviewProps {
  go: (id: string) => void;
}

// Overview is the primary tab: hero, blurb, and the install / quick-start /
// comparison / features sections composed together.
export function Overview({ go }: OverviewProps) {
  return (
    <section className="view active" id="view-overview">
      <Hero go={go} />
      <p className="muted">{LIB.blurb}</p>
      <Install />
      <QuickStart />
      <NodeVsGo />
      <Features />
      <div className="note">Full API reference &amp; runnable examples: <a href="#docs" onClick={(e) => { e.preventDefault(); go('docs'); }}>the Docs tab</a>.</div>
    </section>
  );
}
