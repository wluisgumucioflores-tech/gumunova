import { es } from '../../i18n/es'

export const MAX_IMAGE_BYTES = 5 * 1024 * 1024
export const ACCEPTED_IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/webp']

/**
 * Quick check that saves a round trip. The backend checks the size and the real content
 * type again and is the authority. Returns a Spanish message, or null when the file is fine.
 */
export function validateImageFile(file: File): string | null {
  if (!ACCEPTED_IMAGE_TYPES.includes(file.type)) return es.image.typeClient
  if (file.size > MAX_IMAGE_BYTES) return es.image.tooLargeClient
  return null
}
