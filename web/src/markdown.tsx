// A small Markdown renderer for what forges and agents write: descriptions,
// notes, and drafts. It builds Preact elements and never sets HTML, so text
// from a merge request's author cannot become markup or script. Raw HTML in
// the source shows as the text it is. Links go out only to http, https, and
// mailto.
import { createContext, type ComponentChildren, type JSX } from 'preact'
import { useContext } from 'preact/hooks'

export interface Refs {
  // base is the project's web URL, for GitLab references such as !12 and #3.
  base?: string
 kind?: string
}

// RefsContext gives every Markdown under a change that change's project.
export const RefsContext = createContext<Refs>({})

// projectURL is the project's web URL from a merge request's.
export function projectURL(mrURL: string): string | undefined {
  const i = mrURL.includes('/-/') ? mrURL.indexOf('/-/') : mrURL.indexOf('/pulls/')
  return i > 0 ? mrURL.slice(0, i) : undefined
}

// commitURL is a commit's page on the project's forge, when the project is
// known.
export function commitURL(refs: Refs, sha: string): string | undefined {
  if (!refs.base) return undefined
  return refs.kind === 'gitlab' ? `${refs.base}/-/commit/${sha}` : `${refs.base}/commit/${sha}`
}

export function Markdown({ text, refs, class: cls = '' }: { text: string; refs?: Refs; class?: string }) {
  const ctx = useContext(RefsContext)
  refs ??= ctx
  return <div class={`md ${cls}`}>{blocks(text.replace(/\r\n?/g, '\n'), refs)}</div>
}

type Block = JSX.Element

const fenceRE = /^ {0,3}(`{3,}|~{3,})\s*([\w+-]*)/
const headingRE = /^ {0,3}(#{1,6})\s+(.*?)\s*#*\s*$/
const listRE = /^( *)([-*+]|\d{1,9}[.)])\s+(.*)$/
const hrRE = /^ {0,3}([-*_])( *\1){2,}\s*$/
const tableSepRE = /^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$/

function blocks(src: string, refs: Refs): Block[] {
  const lines = src.split('\n')
  const out: Block[] = []
  let i = 0
  let key = 0
  while (i < lines.length) {
    const line = lines[i]
    if (line.trim() === '') {
      i++
      continue
    }
    let m = fenceRE.exec(line)
    if (m) {
      const fence = m[1]
      const lang = m[2]
      const body: string[] = []
      i++
      while (i < lines.length && !lines[i].trimStart().startsWith(fence)) body.push(lines[i++])
      i++
      out.push(
        <pre class={`md-code ${lang === 'suggestion' || lang.startsWith('suggestion') ? 'md-suggestion' : ''}`} key={key++}>
          {lang && <span class="md-lang">{lang}</span>}
          <code>{body.join('\n')}</code>
        </pre>,
      )
      continue
    }
    if ((m = headingRE.exec(line))) {
      const level = Math.min(m[1].length + 2, 6) // the page owns h1 and h2
      const H = `h${level}` as 'h3'
      out.push(<H key={key++}>{inline(m[2], refs)}</H>)
      i++
      continue
    }
    if (hrRE.test(line)) {
      out.push(<hr key={key++} />)
      i++
      continue
    }
    if (/^ {0,3}>/.test(line)) {
      const body: string[] = []
      while (i < lines.length && /^ {0,3}>/.test(lines[i])) body.push(lines[i++].replace(/^ {0,3}> ?/, ''))
      out.push(<blockquote key={key++}>{blocks(body.join('\n'), refs)}</blockquote>)
      continue
    }
    if (line.includes('|') && i + 1 < lines.length && tableSepRE.test(lines[i + 1])) {
      const head = cells(line)
      i += 2
      const rows: string[][] = []
      while (i < lines.length && lines[i].includes('|') && lines[i].trim() !== '') rows.push(cells(lines[i++]))
      out.push(
        <div class="md-table" key={key++}>
          <table>
            <thead><tr>{head.map((c, j) => <th key={j}>{inline(c, refs)}</th>)}</tr></thead>
            <tbody>{rows.map((r, j) => <tr key={j}>{r.map((c, k) => <td key={k}>{inline(c, refs)}</td>)}</tr>)}</tbody>
          </table>
        </div>,
      )
      continue
    }
    if (listRE.test(line)) {
      const [list, next] = parseList(lines, i, refs)
      out.push(<Fragmentish key={key++}>{list}</Fragmentish>)
      i = next
      continue
    }
    const para: string[] = []
    while (
      i < lines.length && lines[i].trim() !== '' && !fenceRE.test(lines[i]) && !headingRE.test(lines[i]) &&
      !/^ {0,3}>/.test(lines[i]) && !(para.length > 0 && listRE.test(lines[i])) && !hrRE.test(lines[i])
    ) para.push(lines[i++])
    out.push(<p key={key++}>{inline(para.join('\n'), refs)}</p>)
  }
  return out
}

function Fragmentish({ children }: { children: ComponentChildren }) {
  return <>{children}</>
}

function cells(row: string): string[] {
  return row.trim().replace(/^\|/, '').replace(/\|$/, '').split('|').map((c) => c.trim())
}

// parseList reads a list starting at lines[i], with nested lists by indent.
function parseList(lines: string[], i: number, refs: Refs): [Block, number] {
  const first = listRE.exec(lines[i])!
  const indent = first[1].length
  const ordered = /\d/.test(first[2])
  const items: ComponentChildren[] = []
  while (i < lines.length) {
    const m = listRE.exec(lines[i])
    if (!m || m[1].length < indent) break
    if (m[1].length > indent) {
      const [nested, next] = parseList(lines, i, refs)
      items[items.length - 1] = <>{items[items.length - 1]}{nested}</>
      i = next
      continue
    }
    const text = [m[3]]
    i++
    while (i < lines.length && lines[i].trim() !== '' && !listRE.test(lines[i]) && /^\s+/.test(lines[i])) text.push(lines[i++].trim())
    // A blank line inside a list keeps the list going when the next line is
    // another item.
    if (i < lines.length && lines[i].trim() === '' && i + 1 < lines.length && listRE.test(lines[i + 1]) && listRE.exec(lines[i + 1])![1].length >= indent) i++
    const task = /^\[([ xX])\]\s+(.*)$/s.exec(text.join(' '))
    items.push(task ? <><span class="md-task">{task[1] === ' ' ? '☐' : '☑'}</span> {inline(task[2], refs)}</> : inline(text.join('\n'), refs))
  }
  const L = ordered ? 'ol' : 'ul'
  return [<L>{items.map((it, k) => <li key={k}>{it}</li>)}</L>, i]
}

// safeHref returns href if it is a link vuoro may open, or undefined.
export function safeHref(href: string, refs: Refs): string | undefined {
  const h = href.trim()
  if (/^(https?:|mailto:)/i.test(h)) return h
  if (refs.base && /^\/?[\w./#?=&%-]+$/.test(h) && !h.startsWith('//')) {
    return refs.base.replace(/\/$/, '') + '/' + h.replace(/^\//, '')
  }
  return undefined
}

// Inline spans, in priority order. Each pattern's first group is the content.
const inlineRE =
  /(`+)([\s\S]*?[^`])\1(?!`)|!?\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)|\*\*([\s\S]+?)\*\*|__([\s\S]+?)__|~~([\s\S]+?)~~|(?<![\w*])\*(?!\s)([\s\S]+?)(?<!\s)\*(?!\*)|(?<![\w_])_(?!\s)([\s\S]+?)(?<!\s)_(?![\w_])|(https?:\/\/[^\s<>()]+[^\s<>().,;:!?'"])|(?<![\w/!#])([!#])(\d+)\b|(\n)/g

export function inline(src: string, refs: Refs): ComponentChildren[] {
  const out: ComponentChildren[] = []
  let last = 0
  let k = 0
  for (const m of src.matchAll(inlineRE)) {
    const at = m.index ?? 0
    if (at > last) out.push(src.slice(last, at))
    last = at + m[0].length
    if (m[1]) out.push(<code key={k++}>{m[2].replace(/^ (.*) $/, '$1')}</code>)
    else if (m[4] !== undefined) {
      const href = safeHref(m[4], refs)
      const label = inline(m[3], refs)
      out.push(href ? <a href={href} target="_blank" rel="noreferrer noopener" key={k++}>{label.length ? label : href}</a> : <span key={k++}>{label}</span>)
    } else if (m[5] !== undefined || m[6] !== undefined) out.push(<strong key={k++}>{inline(m[5] ?? m[6], refs)}</strong>)
    else if (m[7] !== undefined) out.push(<s key={k++}>{inline(m[7], refs)}</s>)
    else if (m[8] !== undefined || m[9] !== undefined) out.push(<em key={k++}>{inline(m[8] ?? m[9], refs)}</em>)
    else if (m[10]) out.push(<a href={m[10]} target="_blank" rel="noreferrer noopener" key={k++}>{m[10]}</a>)
    else if (m[11]) {
      const kind = m[11] === '!' ? 'merge_requests' : 'issues'
      const gitlab = refs.kind !== 'forgejo'
      out.push(refs.base && (gitlab || m[11] === '#') ? <a href={`${refs.base.replace(/\/$/, '')}/${gitlab ? '-/' : ''}${kind}/${m[12]}`} target="_blank" rel="noreferrer noopener" key={k++}>{m[11]}{m[12]}</a> : m[0])
    } else if (m[13]) out.push(<br key={k++} />)
  }
  if (last < src.length) out.push(src.slice(last))
  return out
}
