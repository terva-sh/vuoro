// A fix saved as a patch, shown again in the diff viewer after its branch and
// the editor's repository are gone (D51).
import { createContext } from 'preact'
import { useContext, useEffect, useState } from 'preact/hooks'
import { useAction } from './action'
import { api } from './api'
import { diffStat } from './diff'
import { DiffBrowser, Stat, useFilePicker } from './diffview'
import type { Patch, PatchDetail } from './types'
import { short } from './ui'
import { WriteContext } from './write'

// PatchesContext is the run whose saved patches SavedPatch reads.
export const PatchesContext = createContext<{ runID: string; patches: Patch[] } | undefined>(undefined)

// SavedPatch offers the diff of a commit the run saved, read when first
// shown. A commit the run did not save shows nothing.
export function SavedPatch({ sha, open: initially = false }: { sha: string; open?: boolean }) {
  const saved = useContext(PatchesContext)
  const meta = saved?.patches.find((p) => p.sha === sha.toLowerCase())
  const [open, setOpen] = useState(initially)
  if (!saved || !meta) return null
  if (meta.too_large) {
    return (
      <span class="saved-patch muted">
        {' '}· its patch was too large to keep{meta.files?.length ? `; it touched ${meta.files.join(', ')}` : ''}
      </span>
    )
  }
  return (
    <>
      {' '}<button class="link saved-patch" aria-expanded={open} onClick={() => setOpen(!open)}>{open ? 'Hide the diff' : 'Show the diff'}</button>
      {open && <PatchDiff runID={saved.runID} sha={meta.sha} />}
    </>
  )
}

function PatchDiff({ runID, sha }: { runID: string; sha: string }) {
  const [p, setP] = useState<PatchDetail>()
  const [copied, setCopied] = useState(false)
  const act = useAction()
  const prefix = `patch-${sha.slice(0, 12)}-`
  const { picked, pick } = useFilePicker(prefix)
  useEffect(() => { void act.run(async () => setP(await api<PatchDetail>(`/api/runs/${runID}/patches/${sha}`))) }, [runID, sha])
  if (act.error) return <p class="alert" role="alert">⚠ {act.error}</p>
  if (!p) return <p class="muted">⧗ Reading the patch…</p>
  const total = p.diff.reduce((t, f) => { const s = diffStat(f.patch); return { add: t.add + s.add, del: t.del + s.del } }, { add: 0, del: 0 })
  const copy = () => void navigator.clipboard?.writeText(p.patch ?? '').then(() => setCopied(true), () => undefined)
  return (
    <div class="patch-diff">
      <h4 class="editor-diff-head">
        <span class="mono">{short(p.sha)}</span> {p.subject} · {p.diff.length} file{p.diff.length === 1 ? '' : 's'} <Stat {...total} />
        <button class="push" onClick={copy} title="A patch from git format-patch, which git am applies">{copied ? 'Copied' : 'Copy the patch'}</button>
      </h4>
      {/* A saved fix is not the change's diff, so nothing here comments on the forge. */}
      <WriteContext.Provider value={undefined}>
        <DiffBrowser files={p.diff} picked={picked} onPick={pick} idPrefix={prefix} />
      </WriteContext.Provider>
    </div>
  )
}
