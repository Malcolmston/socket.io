import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { DocsView } from '../../../src/components/DocsView';
import type { DocIndex } from 'go-ui';

// A minimal DocIndex the stubbed fetch returns for DocsApp's doc.json request.
const DOC_INDEX: DocIndex = {
  module: 'github.com/malcolmston/socketio',
  packages: [
    {
      importPath: 'github.com/malcolmston/socketio',
      name: 'socketio',
      synopsis: 'Package socketio is a Socket.IO server and client.',
      doc: 'Package socketio is a Socket.IO server and client.',
      consts: [],
      vars: [],
      types: [
        {
          name: 'Server',
          signature: 'type Server struct{}',
          doc: 'Server is a Socket.IO server.',
          consts: [],
          vars: [],
          funcs: [],
          methods: [],
        },
      ],
      funcs: [{ name: 'NewServer', signature: 'func NewServer() *Server', doc: 'NewServer creates a server.' }],
    },
  ],
};

describe('DocsView', () => {
  beforeEach(() => {
    // DocsApp fetches doc.json; return the small index. VersionBadge also fetches
    // (releases) — leave any non-doc request pending so it never resolves.
    global.fetch = vi.fn((input: RequestInfo | URL) => {
      if (String(input).includes('doc.json')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve(DOC_INDEX) } as Response);
      }
      return new Promise<Response>(() => {});
    }) as unknown as typeof fetch;
  });

  it('renders the inline React API reference from the fetched doc.json', async () => {
    const { container } = render(<DocsView />);
    expect(container.querySelector('#view-docs')).not.toBeNull();
    expect(
      screen.getByRole('heading', { level: 2, name: /API documentation/ }),
    ).toBeInTheDocument();

    // DocsApp fetches asynchronously, then renders the package view + symbols.
    expect(await screen.findByRole('heading', { name: /package socketio/ })).toBeInTheDocument();
    expect(container.querySelector('#sym-NewServer'), 'func NewServer symbol card').not.toBeNull();
    expect(container.querySelector('#sym-Server'), 'type Server symbol card').not.toBeNull();

    // The secondary link to the raw generated static HTML remains.
    expect(screen.getByRole('link', { name: /Raw generated HTML/ })).toHaveAttribute('href', './api/');
  });
});
