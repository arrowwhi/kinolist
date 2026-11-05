import test from 'node:test'
import assert from 'node:assert/strict'
import { greeting } from '../src/pages/home/model.ts'

test('home greets the current user and handles an absent or empty name', () => {
  assert.equal(greeting(' Иван '), 'Привет, Иван!')
  for (const name of [undefined, '', '   '])
    assert.equal(greeting(name), 'Привет!')
})
