/**
 * API client for GAD System Demo
 *
 * In a real implementation, this would include:
 * - Error handling and retry logic
 * - Authentication tokens
 * - Request/response interceptors
 * - Cache management
 */

const API_BASE = '/api';

export async function fetchAPI<T>(endpoint: string): Promise<T> {
  const response = await fetch(`${API_BASE}${endpoint}`);

  if (!response.ok) {
    throw new Error(`API error: ${response.statusText}`);
  }

  return response.json();
}
