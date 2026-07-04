import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ReleasesView } from '../../../src/components/ReleasesView';

describe('ReleasesView', () => {
  beforeEach(() => {
    // ReleaseList/LibReleases fetch on mount; keep the promise pending.
    global.fetch = vi.fn().mockReturnValue(new Promise(() => {}));
  });

  it('renders the releases heading and the live release list scaffold', () => {
    const { container } = render(<ReleasesView />);
    expect(container.querySelector('#view-releases')).not.toBeNull();
    expect(screen.getByRole('heading', { level: 2, name: /Releases/ })).toBeInTheDocument();
    // The live ReleaseList container is present and scoped to a single repo.
    expect(container.querySelector('#rellist')).not.toBeNull();
    expect(container.querySelectorAll('#rellist .rel-lib').length).toBe(1);
    expect(screen.getByText(/Loading releases/)).toBeInTheDocument();
  });
});
