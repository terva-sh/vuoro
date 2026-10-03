// @vitest-environment jsdom
import { render } from 'preact'
import { describe, expect, it } from 'vitest'
import { Markdown } from './markdown'

const html = (text: string, base?: string) => {
  const root = document.createElement('div')
  render(<Markdown text={text} refs={{ base }} />, root)
  return root
}

describe('Markdown', () => {
  it('renders the structure a description uses', () => {
    const r = html('## Where this sits\n\n**Targets `main` directly.** See !138 and #4.\n\n- one\n- two\n  - nested\n\n```go\nx := 1\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |', 'https://gitlab.com/g/p')
    expect(r.querySelector('h4')?.textContent).toBe('Where this sits')
    expect(r.querySelector('strong code')?.textContent).toBe('main')
    expect([...r.querySelectorAll('a')].map((a) => a.getAttribute('href'))).toEqual([
      'https://gitlab.com/g/p/-/merge_requests/138', 'https://gitlab.com/g/p/-/issues/4',
    ])
    expect(r.querySelectorAll('ul li').length).toBe(3)
    expect(r.querySelector('ul ul li')?.textContent).toBe('nested')
    expect(r.querySelector('pre code')?.textContent).toBe('x := 1')
    expect(r.querySelector('td')?.textContent).toBe('1')
  })

  it('never lets the text become markup or script', () => {
    const r = html('<img src=x onerror=alert(1)> <script>alert(2)</script>\n\n[click](javascript:alert(3)) [data](data:text/html,x)\n\n```\n<b>raw</b>\n```')
    expect(r.querySelector('img, script, b')).toBeNull()
    expect(r.textContent).toContain('<script>alert(2)</script>')
    for (const a of r.querySelectorAll('a')) expect(a.getAttribute('href')).toMatch(/^https?:|^mailto:/)
    expect(r.querySelectorAll('a').length).toBe(0)
  })

  it('links bare URLs without their trailing punctuation', () => {
    const r = html('See https://example.com/a_b. Then go.')
    expect(r.querySelector('a')?.getAttribute('href')).toBe('https://example.com/a_b')
  })
})

describe('Forgejo references', () => {
  it('links # references through Forgejo and leaves GitLab notation as text', () => {
    const root = document.createElement('div')
    render(<Markdown text="See #2 and !7." refs={{ base: 'https://forge.example/o/r', kind: 'forgejo' }} />, root)
    expect([...root.querySelectorAll('a')].map((a) => a.getAttribute('href'))).toEqual(['https://forge.example/o/r/issues/2'])
    expect(root.textContent).toContain('!7')
  })
  it('derives a project URL from both kinds of change URL', async () => {
    const { projectURL } = await import('./markdown')
    expect(projectURL('https://forge.example/o/r/pulls/2')).toBe('https://forge.example/o/r')
    expect(projectURL('https://gitlab.example/g/sub/p/-/merge_requests/2')).toBe('https://gitlab.example/g/sub/p')
  })
})
