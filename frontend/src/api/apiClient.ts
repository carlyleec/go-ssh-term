export class ApiError extends Error {
  constructor(
    public status: number,
    public body: unknown,
    message?: string,
  ) {
    const serverMessage =
      typeof body === 'object' &&
      body !== null &&
      'error' in body &&
      typeof body.error === 'string'
        ? body.error
        : undefined
    const hop =
      typeof body === 'object' && body !== null && 'hop' in body
        ? body.hop
        : undefined
    const prefix =
      hop === 'bastion' ? 'Bastion: ' : hop === 'target' ? 'Target: ' : ''
    super(
      message ??
        (serverMessage ? prefix + serverMessage : undefined) ??
        'Could not complete the request. Please try again.',
    )
    this.name = 'ApiError'
  }
}

type RequestOptions = RequestInit & { expectedStatus?: number }

async function request<T>(
  url: string,
  { expectedStatus, ...options }: RequestOptions = {},
): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Accept', 'application/json')
  let response: Response
  try {
    response = await fetch(url, {
      credentials: 'same-origin',
      cache: 'no-store',
      ...options,
      headers,
    })
  } catch (error) {
    if (options.signal?.aborted) throw error
    throw new ApiError(
      0,
      null,
      'Could not reach the server. Check your connection and retry.',
    )
  }

  if (!response.ok || (expectedStatus && response.status !== expectedStatus)) {
    const body = await response.json().catch(() => null)
    throw new ApiError(response.status, body)
  }
  if (response.status === 204) return undefined as T

  try {
    return (await response.json()) as T
  } catch (error) {
    if (options.signal?.aborted) throw error
    throw new ApiError(
      response.status,
      null,
      'The server returned an unexpected response.',
    )
  }
}

export const apiClient = {
  get<T>(url: string, options?: RequestOptions): Promise<T> {
    return request<T>(url, options)
  },

  post<T>(url: string, body: unknown, options?: RequestOptions): Promise<T> {
    const headers = new Headers(options?.headers)
    const multipart = body instanceof FormData
    if (!multipart) headers.set('Content-Type', 'application/json')
    return request<T>(url, {
      ...options,
      method: 'POST',
      headers,
      body: multipart ? body : JSON.stringify(body),
    })
  },

  put<T>(url: string, body: unknown, options?: RequestOptions): Promise<T> {
    const headers = new Headers(options?.headers)
    headers.set('Content-Type', 'application/json')
    return request<T>(url, {
      ...options,
      method: 'PUT',
      headers,
      body: JSON.stringify(body),
    })
  },

  delete<T>(url: string, options?: RequestOptions): Promise<T> {
    return request<T>(url, { ...options, method: 'DELETE' })
  },
}

export default apiClient
