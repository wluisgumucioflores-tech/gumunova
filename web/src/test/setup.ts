import '@testing-library/jest-dom/vitest'
import { cleanup, configure } from '@testing-library/react'
import { afterEach, vi } from 'vitest'

// findBy* and waitFor give up after 1 s by default, which is too tight when the machine is busy
// (for example while the Go tests compile). Slow runs should not look like failures.
configure({ asyncUtilTimeout: 4000 })

afterEach(() => {
  cleanup()
})

// jsdom does not implement object URLs, which the image preview uses.
URL.createObjectURL = vi.fn(() => 'blob:preview')
URL.revokeObjectURL = vi.fn()
