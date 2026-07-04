import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { Html } from '../../../src/components/Html';

describe('Html', () => {
  it('renders raw markup into a span by default', () => {
    const { container } = render(<Html html='<i class="fa-solid fa-bolt"></i>' />);
    const span = container.querySelector('span');
    expect(span).not.toBeNull();
    expect(span!.querySelector('i.fa-bolt')).not.toBeNull();
  });

  it('renders into a custom tag and forwards props', () => {
    const { container } = render(<Html tag="li" html="<b>hi</b>" className="feat-item" />);
    const li = container.querySelector('li.feat-item');
    expect(li).not.toBeNull();
    expect(li!.querySelector('b')?.textContent).toBe('hi');
  });
});
