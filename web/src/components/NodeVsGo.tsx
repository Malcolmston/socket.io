import { CompareCard, hi } from 'go-ui';
import { LIB } from '../data';
import { SecH } from './SecH';

// NodeVsGo renders the side-by-side Node.js → Go comparison columns.
export function NodeVsGo() {
  return (
    <>
      <SecH id="cmp">Node.js → Go</SecH>
      <div className="compare">
        <CompareCard name="Node.js" color="var(--node)" html={hi(LIB.node_code)} />
        <CompareCard name="Go" color="var(--go)" html={hi(LIB.go_code)} />
      </div>
    </>
  );
}
