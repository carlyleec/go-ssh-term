import { mock } from 'bun:test'

// Keep Bun's extra fetch property while replacing only the HTTP call boundary.
export function mockFetch(
  implementation: (
    ...args: Parameters<typeof fetch>
  ) => ReturnType<typeof fetch>,
) {
  return Object.assign(mock(implementation), { preconnect: fetch.preconnect })
}
