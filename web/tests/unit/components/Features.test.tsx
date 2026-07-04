import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Features } from '../../../src/components/Features';
import { LIB } from '../../../src/data';

describe('Features', () => {
  it('renders the Features heading and one bullet per feature', () => {
    const { container } = render(<Features />);
    expect(screen.getByRole('heading', { name: 'Features' })).toBeInTheDocument();
    expect(container.querySelectorAll('ul.feat li').length).toBe(LIB.features.length);
  });
});
