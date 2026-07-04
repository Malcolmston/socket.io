import { CodeBlock, hi } from 'go-ui';
import { LIB } from '../data';
import { SecH } from './SecH';

// QuickStart shows the minimal server snippet plus a "going further" sample
// covering acks, per-socket data and Redis scale-out.
export function QuickStart() {
  return (
    <>
      <SecH id="quick">Quick start</SecH>
      <CodeBlock lang="main.go" html={hi(LIB.go_code)} />

      <SecH id="more">Going further</SecH>
      <CodeBlock lang="go" html={LIB.integrate} />
    </>
  );
}
