import type { ElementType, ReactNode, CSSProperties } from 'react';

export interface SecHProps {
  children: ReactNode;
  h?: ElementType;
  style?: CSSProperties;
  id?: string;
}

// SecH is a section heading with the accent bar used throughout the site.
export function SecH({ children, h = 'h3', style, id }: SecHProps) {
  const H = h;
  return <div className="sec-h" style={style} id={id}><span className="bar" /><H style={{ margin: 0 }}>{children}</H></div>;
}
