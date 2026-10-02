// Fails when the committed web/dist differs from a fresh build of web/src.
// It builds into a temporary directory and never touches web/dist.
import { execFileSync } from 'node:child_process'
import { existsSync, lstatSync, mkdtempSync, readFileSync, readdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

function tree(dir, prefix = '') {
  const files = new Map()
  if (!existsSync(dir)) return files
  for (const name of readdirSync(dir).sort()) {
    const full = join(dir, name)
    const stat = lstatSync(full)
    if (stat.isDirectory()) for (const e of tree(full, prefix + name + '/')) files.set(...e)
    else if (stat.isFile()) files.set(prefix + name, readFileSync(full))
    else throw new Error(`unexpected entry in a build: ${prefix + name}`)
  }
  return files
}

const out = mkdtempSync(join(tmpdir(), 'vuoro-dist-'))
try {
  execFileSync('npx', ['vite', 'build', '--outDir', out, '--emptyOutDir', '--logLevel', 'error'], { stdio: 'inherit' })
  const fresh = tree(out), committed = tree('web/dist')
  const errors = []
  for (const [name, bytes] of fresh) {
    if (!committed.has(name)) errors.push(`missing from web/dist: ${name}`)
    else if (!bytes.equals(committed.get(name))) errors.push(`differs from a fresh build: web/dist/${name}`)
  }
  for (const name of committed.keys()) if (!fresh.has(name)) errors.push(`not in a fresh build: web/dist/${name}`)
  if (errors.length) {
    console.error(errors.join('\n') + '\nRun `just web-build` and commit web/dist.')
    process.exit(1)
  }
  console.log(`web/dist matches a fresh build (${fresh.size} files)`)
} finally {
  rmSync(out, { recursive: true, force: true })
}
