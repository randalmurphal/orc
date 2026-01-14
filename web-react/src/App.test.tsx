import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import App from './App';

describe('App', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('renders header with title', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ status: 'ok' }),
    } as Response);

    render(<App />);
    expect(screen.getByText('Orc - Task Orchestrator')).toBeInTheDocument();
    expect(screen.getByText('React 19 Migration')).toBeInTheDocument();
    // Wait for fetch to complete to avoid act warnings
    await waitFor(() => {
      expect(screen.getByText(/API Status/)).toBeInTheDocument();
    });
  });

  it('shows loading state initially', () => {
    vi.mocked(fetch).mockImplementation(
      () => new Promise(() => {}) // Never resolves
    );

    render(<App />);
    expect(screen.getByText('Checking API...')).toBeInTheDocument();
  });

  it('displays health status on successful API call', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ status: 'ok', version: '1.0.0' }),
    } as Response);

    render(<App />);
    await waitFor(() => {
      expect(screen.getByText(/API Status: ok/)).toBeInTheDocument();
      expect(screen.getByText(/v1.0.0/)).toBeInTheDocument();
    });
  });

  it('displays error on API failure', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      status: 500,
    } as Response);

    render(<App />);
    await waitFor(() => {
      expect(screen.getByText(/API Error: HTTP 500/)).toBeInTheDocument();
    });
  });

  it('displays error on network failure', async () => {
    vi.mocked(fetch).mockRejectedValueOnce(new Error('Network error'));

    render(<App />);
    await waitFor(() => {
      expect(screen.getByText(/API Error: Network error/)).toBeInTheDocument();
    });
  });

  it('renders project structure section', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ status: 'ok' }),
    } as Response);

    render(<App />);
    expect(screen.getByText('Project Structure')).toBeInTheDocument();
    expect(screen.getByText('src/components/')).toBeInTheDocument();
    // Wait for fetch to complete to avoid act warnings
    await waitFor(() => {
      expect(screen.getByText(/API Status/)).toBeInTheDocument();
    });
  });
});
