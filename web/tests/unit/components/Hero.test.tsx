import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Hero } from '../../../src/components/Hero';
import { LIB } from '../../../src/data';

describe('Hero', () => {
  beforeEach(() => {
    // VersionBadge fetches on mount.
    global.fetch = vi.fn().mockReturnValue(new Promise(() => {}));
  });

  it('renders the library name, package path and tagline', () => {
    render(<Hero go={() => {}} />);
    expect(screen.getByRole('heading', { level: 1, name: /Socket\.IO/ })).toBeInTheDocument();
    expect(screen.getByText(LIB.pkg)).toBeInTheDocument();
    expect(screen.getByText(LIB.tagline)).toBeInTheDocument();
  });

  it('renders an external GitHub link opening in a new tab', () => {
    render(<Hero go={() => {}} />);
    const github = screen.getAllByRole('link', { name: /GitHub/ })[0];
    expect(github).toHaveAttribute('href', LIB.repo);
    expect(github).toHaveAttribute('target', '_blank');
    expect(github).toHaveAttribute('rel', expect.stringContaining('noopener'));
  });

  it('navigates to the docs tab when the CTA is clicked', async () => {
    const go = vi.fn();
    render(<Hero go={go} />);
    await userEvent.click(screen.getByRole('link', { name: /Read the docs/ }));
    expect(go).toHaveBeenCalledWith('docs');
  });
});
