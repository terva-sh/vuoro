// Capture the README images from scene.ts. `just screenshots` runs it.
//
// Chromium loads the committed web/dist, and every request it makes is
// answered here: the page's files from web/dist, and each /api/ route from
// the scene. No vuoro binary, forge, token, or agent runs, and nothing leaves
// the machine. An /api/ request the scene has no answer for fails the
// capture, so a screen that starts reading a new route is not shot half
// loaded.
//
// The clock, timezone, locale, viewport, and scheme are fixed, so two runs on
// one machine write the same bytes. Text is drawn with the machine's own
// fonts, so another machine writes slightly different images. That is why
// this is not a CI gate.
import { chromium, type Page, type Route } from '@playwright/test'
import { mkdir, readFile } from 'node:fs/promises'
import { dirname, extname, join, normalize, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import * as scene from './scene.ts'

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const dist = join(repo, 'web/dist')

function argument(name: string, fallback: string): string {
  const i = process.argv.indexOf(name)
  return i < 0 ? fallback : process.argv[i + 1] ?? fallback
}

const out = resolve(repo, argument('--out', 'docs/images'))
const width = Number(argument('--width', '1440'))
const height = Number(argument('--height', '900'))
if (!Number.isInteger(width) || width < 1 || !Number.isInteger(height) || height < 1) {
  throw new Error('--width and --height must be positive integers')
}

// A reserved name, which never resolves, so a request the routes miss fails
// rather than reaching anything.
const origin = 'http://vuoro.invalid'

const types: Record<string, string> = {
  '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.png': 'image/png', '.woff2': 'font/woff2',
}

type Scheme = 'light' | 'dark'

// answer is the scene's reply to one /api/ request, or undefined for none.
function answer(method: string, path: string, scheme: Scheme): unknown {
  const routes: Record<string, unknown> = {
    'GET /api/hello': scene.hello,
    'GET /api/setup': scene.setup(scheme),
    'GET /api/runs': scene.runs,
    'GET /api/local': scene.localCached,
    'GET /api/inbox': scene.inbox,
    'POST /api/inbox/refresh': scene.inbox,
    'POST /api/changes': scene.change,
    [`GET /api/runs/${scene.run.id}`]: scene.run,
    [`GET /api/runs/${scene.run.id}/editor`]: scene.editor,
    [`GET /api/runs/${scene.run.id}/editor/delivery`]: scene.delivery,
  }
  return routes[`${method} ${path}`]
}

async function serve(route: Route, scheme: Scheme, missed: string[]) {
  const request = route.request()
  const u = new URL(request.url())
  if (u.origin !== origin) return route.abort()
  if (u.pathname.startsWith('/api/')) {
    const body = answer(request.method(), u.pathname, scheme)
    if (body === undefined) {
      missed.push(`${request.method()} ${u.pathname}`)
      return route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: 'no fixture' }) })
    }
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify(body) })
  }
  const file = normalize(join(dist, u.pathname === '/' ? 'index.html' : u.pathname))
  if (!file.startsWith(dist)) return route.abort()
  const bytes = await readFile(file).catch(() => undefined)
  if (!bytes) return route.fulfill({ status: 404 })
  return route.fulfill({ body: bytes, contentType: types[extname(file)] ?? 'application/octet-stream' })
}

// shoot writes the viewport, unless a request went unanswered, when the
// screen may be half loaded and nothing is written.
async function shoot(page: Page, name: string, missed: string[]) {
  await page.evaluate(() => document.fonts.ready)
  if (missed.length > 0) throw new Error(`no fixture in scene.ts for: ${[...new Set(missed)].join(', ')}`)
  const path = join(out, `${name}.png`)
  await page.screenshot({ path, animations: 'disabled', caret: 'hide' })
  console.log(`  ${path.slice(repo.length + 1)}`)
}

// Without these, the antialiasing of a rounded border in the sticky header,
// its own compositor layer, differed between runs by a few pixels.
const rasterFlags = [
  '--disable-gpu', '--disable-partial-raster', '--disable-skia-runtime-opts', '--run-all-compositor-stages-before-draw',
  '--force-color-profile=srgb', '--font-render-hinting=none', '--disable-lcd-text',
]

async function capture(scheme: Scheme) {
  const browser = await chromium.launch({ args: rasterFlags })
  const missed: string[] = []
  try {
    const context = await browser.newContext({
      viewport: { width, height }, deviceScaleFactor: 1, colorScheme: scheme,
      locale: 'en-US', timezoneId: 'UTC', reducedMotion: 'reduce',
    })
    await context.route('**/*', (route) => serve(route, scheme, missed))
    const page = await context.newPage()
    // Date is fixed and timers still run, so the inbox's poll and the run's
    // poll read the same scene again without moving any "ago".
    await page.clock.setFixedTime(scene.now)
    await page.goto(`${origin}/#token=screenshots`)

    // The inbox, with a change that waits on replies selected.
    const inbox = page.locator('section.inbox[aria-busy="false"]')
    await inbox.waitFor()
    await page.locator('.inbox-row', { hasText: 'Explain the cache eviction order' }).click()
    await page.locator('.inbox-detail h2', { hasText: 'Explain the cache eviction order' }).waitFor()
    await shoot(page, `inbox-${scheme}`, missed)

    // The change opened from the inbox, at its Review tab, one finding open.
    await page.locator('.inbox-row', { hasText: 'Stream invoice exports' }).click()
    await page.getByRole('button', { name: 'Open in vuoro' }).click()
    await page.getByRole('tab', { name: /^Review/ }).click()
    const finding = page.locator('details.finding', { hasText: 'An error from the cursor reads as the end of the export' })
    await finding.locator('summary').click()
    await finding.locator('.finding-body').waitFor()
    await page.locator('.editor-commits').waitFor()
    await page.mouse.move(0, 0)
    await shoot(page, `review-${scheme}`, missed)

    // The editor's fixes, offered back as suggestions or as a draft change.
    await page.locator('.run-section', { has: page.locator('.editor-panel') }).evaluate((e) => e.scrollIntoView({ block: 'start' }))
    await shoot(page, `editor-${scheme}`, missed)

    await context.close()
  } finally {
    await browser.close()
  }
}

await mkdir(out, { recursive: true })
for (const scheme of ['light', 'dark'] as const) await capture(scheme)
