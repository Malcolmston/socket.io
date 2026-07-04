import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QuickStart } from '../../../src/components/QuickStart';

describe('QuickStart', () => {
  it('renders the Quick start and Going further headings with code', () => {
    const { container } = render(<QuickStart />);
    expect(screen.getByRole('heading', { name: 'Quick start' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Going further' })).toBeInTheDocument();
    // Two code blocks are rendered.
    expect(container.querySelectorAll('.code').length).toBe(2);
    expect(container.textContent).toContain('socketio.New()');
  });
});
