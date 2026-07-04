import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Install } from '../../../src/components/Install';
import { LIB } from '../../../src/data';

describe('Install', () => {
  it('renders the Install heading and the go get command', () => {
    render(<Install />);
    expect(screen.getByRole('heading', { name: 'Install' })).toBeInTheDocument();
    expect(screen.getByText(new RegExp(`go get ${LIB.pkg}`))).toBeInTheDocument();
  });
});
