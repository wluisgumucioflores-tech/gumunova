import { describe, expect, it } from 'vitest'
import { codeFromScan, isLocalAddress, labelUrl } from './labelLink'

describe('labelUrl', () => {
  it('points the QR at /producto/<code>', () => {
    expect(labelUrl('P-1A2B3C4D5E6F', 'https://gumunova.example.com')).toBe('https://gumunova.example.com/producto/P-1A2B3C4D5E6F')
  })

  it('uses the address the app is open at when none is configured', () => {
    expect(labelUrl('R-0123456789AB')).toBe(`${window.location.origin}/producto/R-0123456789AB`)
  })
})

describe('codeFromScan', () => {
  it('takes the code out of the address a QR carries', () => {
    expect(codeFromScan('https://gumunova.example.com/producto/P-1A2B3C4D5E6F')).toBe('P-1A2B3C4D5E6F')
    expect(codeFromScan(labelUrl('R-0123456789AB', 'http://192.168.0.5:5173'))).toBe('R-0123456789AB')
  })

  it('leaves a bare code as it is, without the whitespace a scanner adds', () => {
    expect(codeFromScan('P-1A2B3C4D5E6F\n')).toBe('P-1A2B3C4D5E6F')
    expect(codeFromScan('  r-0123456789ab ')).toBe('r-0123456789ab')
  })

  it('does not invent a code from an unrelated address', () => {
    expect(codeFromScan('https://example.com/otra?x=1')).toBe('https://example.com/otra?x=1')
  })
})

describe('isLocalAddress', () => {
  it.each(['http://localhost:5173', 'http://127.0.0.1:5173', 'http://app.localhost'])('%s only works on this computer', (address) => {
    expect(isLocalAddress(address)).toBe(true)
  })

  it.each(['https://gumunova.example.com', 'http://192.168.0.5:5173'])('%s can be reached from a phone', (address) => {
    expect(isLocalAddress(address)).toBe(false)
  })
})
