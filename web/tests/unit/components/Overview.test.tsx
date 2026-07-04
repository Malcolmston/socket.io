import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Overview } from '../../../src/components/Overview';
import { LIB } from '../../../src/data';

describe('Overview', () => {
  beforeEach(() => {
    // VersionBadge (via Hero) fetches on mount.
    global.fetch = vi.fn().mockReturnValue(new Promise(() => {}));
  });

  it('renders the single active overview view with all sections', () => {
    const { container } = render(<Overview go={() => {}} />);
    expect(container.querySelector('#view-overview')).not.toBeNull();
    expect(container.querySelectorAll('.view.active').length).toBe(1);
    expect(screen.getByRole('heading', { level: 1, name: /Socket\.IO/ })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Install' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Quick start' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Features' })).toBeInTheDocument();
    expect(screen.getByText(LIB.blurb)).toBeInTheDocument();
  });

  it('navigates to docs when the note link is clicked', async () => {
    const go = vi.fn();
    render(<Overview go={go} />);
    await userEvent.click(screen.getByRole('link', { name: /the Docs tab/ }));
    expect(go).toHaveBeenCalledWith('docs');
  });
});
