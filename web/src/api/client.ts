import { es, type ApiErrorCode, type FieldErrorCode } from '../i18n/es'

export interface FieldError {
  field: string
  code: string
}

/** An error answered by the API (stable code) or a failure to reach it. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: FieldError[]

  constructor(status: number, code: string, fields: FieldError[] = []) {
    super(`API error ${status}: ${code}`)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.fields = fields
  }
}

/** True when a request was cancelled on purpose (for example, the screen changed). */
export function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

interface ErrorBody {
  error?: { code?: string; fields?: FieldError[] }
}

const BASE_URL = '/api/v1'

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  return (await requestWithResponse<T>(path, init)).data
}

/** Like request, but also returns the response, for callers that need its status or headers. */
export async function requestWithResponse<T>(
  path: string,
  init: RequestInit = {},
): Promise<{ data: T; response: Response }> {
  let response: Response
  try {
    response = await fetch(`${BASE_URL}${path}`, init)
  } catch (error) {
    if (isAbortError(error)) throw error
    throw new ApiError(0, 'network')
  }

  if (!response.ok) {
    let body: ErrorBody = {}
    try {
      body = (await response.json()) as ErrorBody
    } catch {
      // A non-JSON error (for example from a proxy) is reported with a generic code.
    }
    throw new ApiError(response.status, body.error?.code ?? 'unknown', body.error?.fields ?? [])
  }
  return { data: (await response.json()) as T, response }
}

export function jsonRequest(method: string, body?: unknown, extraHeaders: Record<string, string> = {}): RequestInit {
  return {
    method,
    headers: { 'Content-Type': 'application/json', ...extraHeaders },
    body: body === undefined ? undefined : JSON.stringify(body),
  }
}

/** Spanish message for an API error code. Technical messages are never shown to the user. */
export function messageForApiError(error: unknown): string {
  if (error instanceof ApiError) {
    const code = error.code as ApiErrorCode
    return es.apiErrors[code] ?? es.apiErrors.unknown
  }
  return es.apiErrors.unknown
}

/** Spanish message for a field-level validation code. */
export function messageForFieldError(code: string): string {
  return es.fieldErrors[code as FieldErrorCode] ?? es.fieldErrors.unknown
}
