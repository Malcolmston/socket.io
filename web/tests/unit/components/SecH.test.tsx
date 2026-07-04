import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SecH } from '../../../src/components/SecH';

describe('SecH', () => {
  it('renders an h3 heading with the accent bar by default', () => {
    const { container } = render(<SecH>Install</SecH>);
    expect(screen.getByRole('heading', { level: 3, name: 'Install' })).toBeInTheDocument();
    expect(container.querySelector('.sec-h .bar')).not.toBeNull();
  });

  it('honours a custom heading level and id', () => {
    const { container } = render(<SecH h="h2" id="rel">Releases</SecH>);
    expect(screen.getByRole('heading', { level: 2, name: 'Releases' })).toBeInTheDocument();
    expect(container.querySelector('#rel')).not.toBeNull();
  });
});
