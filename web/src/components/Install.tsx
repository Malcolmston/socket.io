import { CodeBlock } from 'go-ui';
import { LIB } from '../data';
import { SecH } from './SecH';

// Install renders the `go get` command for the module.
export function Install() {
  return (
    <>
      <SecH id="install">Install</SecH>
      <CodeBlock lang="shell" html={`<span class="tok-c">$</span> go get ${LIB.pkg}`} />
    </>
  );
}
