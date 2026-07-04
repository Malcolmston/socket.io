import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { NodeVsGo } from '../../../src/components/NodeVsGo';

describe('NodeVsGo', () => {
  it('renders the comparison heading and both Node.js and Go columns', () => {
    const { container } = render(<NodeVsGo />);
    expect(screen.getByRole('heading', { name: /Node\.js/ })).toBeInTheDocument();
    expect(screen.getByText('Node.js')).toBeInTheDocument();
    expect(screen.getByText('Go')).toBeInTheDocument();
    expect(container.querySelectorAll('.compare .code').length).toBe(2);
  });
});
