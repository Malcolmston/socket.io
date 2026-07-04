import type { CSSProperties } from 'react';
import { LIB } from '../data';
import { SecH } from './SecH';
import { Html } from './Html';

// Features renders the library's feature bullet list.
export function Features() {
  return (
    <>
      <SecH id="feat">Features</SecH>
      <ul className="feat" style={{ '--lib-accent': LIB.accent } as CSSProperties}>
        {LIB.features.map((f, i) => <Html tag="li" html={f} key={i} />)}
      </ul>
    </>
  );
}
