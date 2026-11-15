import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, jsonRequest, messageForApiError, messageForFieldError, request } from './client'
import { es } from '../i18n/es'

function mockFetch(response: Response | Error) {
  const fetchMock = vi.fn(async () => {
    if (response instanceof Error) throw response
    return response
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('request', () => {
  it('returns parsed JSON and prefixes the API base path', async () => {
    const fetchMock = mockFetch(Response.json({ ok: true }))

    await expect(request<{ ok: boolean }>('/products')).resolves.toEqual({ ok: true })
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/products', {})
  })

  it('turns an API error body into an ApiError with code and fields', async () => {
    mockFetch(
      Response.json(
        { error: { code: 'validation_failed', fields: [{ field: 'sku', code: 'required' }] } },
        { status: 422 },
      ),
    )

    const failure = await request('/products').catch((error: unknown) => error)

    expect(failure).toBeInstanceOf(ApiError)
    expect(failure).toMatchObject({
      status: 422,
      code: 'validation_failed',
      fields: [{ field: 'sku', code: 'required' }],
    })
  })

  it('reports a non-JSON error response with the unknown code', async () => {
    mockFetch(new Response('<html>Bad gateway</html>', { status: 502 }))

    await expect(request('/products')).rejects.toMatchObject({ status: 502, code: 'unknown' })
  })

  it('reports an unreachable server with the network code', async () => {
    mockFetch(new TypeError('Failed to fetch'))

    await expect(request('/products')).rejects.toMatchObject({ status: 0, code: 'network' })
  })
})

describe('jsonRequest', () => {
  it('serializes the body as JSON', () => {
    const init = jsonRequest('POST', { sku: 'PVC-01' })
    expect(init.method).toBe('POST')
    expect(init.body).toBe('{"sku":"PVC-01"}')
    expect(init.headers).toEqual({ 'Content-Type': 'application/json' })
  })
})

describe('error messages', () => {
  it('maps known API codes to Spanish text', () => {
    expect(messageForApiError(new ApiError(409, 'sku_already_exists'))).toBe(es.apiErrors.sku_already_exists)
    expect(messageForApiError(new ApiError(0, 'network'))).toBe(es.apiErrors.network)
  })

  it('never shows technical text for unknown failures', () => {
    expect(messageForApiError(new ApiError(500, 'something_new'))).toBe(es.apiErrors.unknown)
    expect(messageForApiError(new Error('ECONNRESET at 10.0.0.1'))).toBe(es.apiErrors.unknown)
  })

  it('maps field codes to Spanish text with a safe fallback', () => {
    expect(messageForFieldError('must_be_positive')).toBe(es.fieldErrors.must_be_positive)
    expect(messageForFieldError('brand_new_code')).toBe(es.fieldErrors.unknown)
  })
})
