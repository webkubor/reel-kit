import test from 'node:test'
import assert from 'node:assert/strict'
import { githubRepo } from '../src/studio/recommendation.mjs'

test('project input accepts GitHub repository URLs only', () => {
  assert.equal(githubRepo('https://github.com/webkubor/dsh-bloom-theme.git'), 'webkubor/dsh-bloom-theme')
  for (const url of ['http://github.com/a/b', 'https://github.com.evil.test/a/b', 'https://github.com/a/b/tree/main', 'https://localhost/a/b', 'https://user:pass@github.com/a/b']) {
    assert.throws(() => githubRepo(url))
  }
})
