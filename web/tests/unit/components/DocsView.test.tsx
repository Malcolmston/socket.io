import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { DocsView } from '../../../src/components/DocsView';
import { LIB } from '../../../src/data';

describe('DocsView', () => {
  it('renders the API reference heading and a link to the generated ./api/ site', () => {
    const { container } = render(<DocsView />);
    expect(container.querySelector('#view-docs')).not.toBeNull();
    expect(screen.getByRole('heading', { level: 2, name: /API reference/ })).toBeInTheDocument();
    const api = screen.getByRole('link', { name: /Open the API reference/ });
    expect(api).toHaveAttribute('href', './api/');
    expect(api).toHaveAttribute('target', '_blank');
    expect(api).toHaveAttribute('rel', expect.stringContaining('noopener'));
  });

  it('links to the GitHub source', () => {
    render(<DocsView />);
    const gh = screen.getByRole('link', { name: /Source on GitHub/ });
    expect(gh).toHaveAttribute('href', LIB.repo);
    expect(gh).toHaveAttribute('target', '_blank');
  });
});
