import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { App } from '../../src/App';

describe('App', () => {
  beforeEach(() => {
    // Components mounted here (VersionBadge/ReleaseList) fetch on mount.
    global.fetch = vi.fn().mockReturnValue(new Promise(() => {}));
    window.location.hash = '';
  });

  afterEach(() => {
    window.location.hash = '';
  });

  it('renders the nav tabs and the overview view by default', () => {
    render(<App />);
    expect(screen.getByRole('link', { name: 'Overview' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Releases' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Docs' })).toBeInTheDocument();
    // Overview hero heading.
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(/Socket\.IO/i);
  });

  it('switches the visible view when location.hash changes', () => {
    render(<App />);
    window.location.hash = '#releases';
    fireEvent(window, new Event('hashchange'));
    expect(screen.getByRole('heading', { level: 2, name: /Releases/ })).toBeInTheDocument();

    window.location.hash = '#docs';
    fireEvent(window, new Event('hashchange'));
    expect(screen.getByRole('heading', { level: 2, name: /API documentation/ })).toBeInTheDocument();
  });

  it('navigates when a nav tab is clicked', () => {
    render(<App />);
    fireEvent.click(screen.getByRole('link', { name: 'Docs' }));
    fireEvent(window, new Event('hashchange'));
    expect(screen.getByRole('heading', { level: 2, name: /API documentation/ })).toBeInTheDocument();
  });
});
