import test from 'node:test'
import assert from 'node:assert/strict'
import { otherTheme, parseTheme } from '../src/shared/lib/theme.ts'

test('only an explicit light value turns the light theme on', () => {
  assert.equal(parseTheme('light'), 'light')
  for (const value of ['dark', '', null, undefined, 'LIGHT', 'sepia'])
    assert.equal(parseTheme(value), 'dark')
})

test('toggle switches between the two themes', () => {
  assert.equal(otherTheme('dark'), 'light')
  assert.equal(otherTheme('light'), 'dark')
})
