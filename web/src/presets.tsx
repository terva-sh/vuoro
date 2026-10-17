// The presets page: the named sets of reviewers a run starts from. vuoro
// ships three (D61), and those are copied rather than edited, so the shipped
// set keeps its meaning across an upgrade. Only the person's own presets are
// in the configuration file, so only those can be changed or removed.
import { useState } from 'preact/hooks'
import { useAction } from './action'
import { post } from './api'
import { EditForm } from './textedit'
import type { Preset, PresetReviewer, PresetRole, Setup } from './types'

// describe is a preset's reviewers on one line, worded as the run picker
// words them.
export function describe(p: Preset): string {
  return p.reviewers.map((r) => r.profile + (r.model ? ` (${r.model})` : '')).join(', ')
}

// copyName is a free name for a copy of p, so copying twice does not collide
// with the first copy.
export function copyName(name: string, taken: string[]): string {
  let out = `${name} copy`
  for (let n = 2; taken.includes(out); n++) out = `${name} copy ${n}`
  return out
}

// role is what the form holds for a synthesizer or verifier, and what it
// sends. One naming neither a tool nor a model is left out, so the run's own
// defaults apply (D11, D31). The server trims and drops it too; this keeps
// the request honest rather than relying on that.
const role = (tool: string, model: string): PresetRole | undefined =>
  tool.trim() || model.trim() ? { tool: tool.trim() || undefined, model: model.trim() || undefined } : undefined

// PresetEditor writes one preset. original names the preset it replaces, and
// is undefined for a new one, which is also what a copy of a built-in is.
function PresetEditor({ setup, initial, original, onSaved, onCancel }: {
  setup: Setup; initial: Preset; original?: string; onSaved: (s: Setup) => void; onCancel: () => void
}) {
  const [name, setName] = useState(initial.name)
  const [reviewers, setReviewers] = useState<PresetReviewer[]>(initial.reviewers.map((r) => ({ ...r })))
  const [synthTool, setSynthTool] = useState(initial.synthesizer?.tool ?? '')
  const [synthModel, setSynthModel] = useState(initial.synthesizer?.model ?? '')
  const [verifyTool, setVerifyTool] = useState(initial.verifier?.tool ?? '')
  const [verifyModel, setVerifyModel] = useState(initial.verifier?.model ?? '')
  const { busy, error, run } = useAction()

  const edit = (i: number, change: Partial<PresetReviewer>) =>
    setReviewers((rs) => rs.map((r, n) => (n === i ? { ...r, ...change } : r)))
  const profiles = setup.profiles ?? []
  const canSave = name.trim() !== '' && reviewers.length > 0 && reviewers.every((r) => r.profile.trim() !== '')

  const save = () => run(async () => {
    const preset: Preset = {
      name: name.trim(),
      reviewers: reviewers.map((r) => ({ profile: r.profile, tool: r.tool || undefined, model: r.model || undefined })),
      synthesizer: role(synthTool, synthModel),
      verifier: role(verifyTool, verifyModel),
    }
    onSaved(await post<Setup>('/api/setup/presets', { base: setup.base, original: original ?? '', preset }))
  })

  return (
    <EditForm class="preset-editor" onSave={save} onCancel={onCancel} busy={busy} canSave={canSave}
      save={original ? 'Save' : 'Add preset'} error={error}
      hint={canSave ? undefined : 'A preset needs a name and at least one reviewer'}
      note="A run starts one session for each reviewer. More than one gets a synthesizer, and every run gets a verifier.">
      <label class="field">Name
        <input value={name} placeholder="thorough" onInput={(e) => setName((e.target as HTMLInputElement).value)} />
      </label>
      <ul class="preset-reviewers">
        {reviewers.map((r, i) => (
          <li class="row" key={i}>
            <label class="field">Reviews for
              <select value={r.profile} onChange={(e) => edit(i, { profile: (e.target as HTMLSelectElement).value })}>
                {profiles.map((p) => <option value={p.name} key={p.name}>{p.title || p.name}</option>)}
              </select>
            </label>
            <ToolField label="On" value={r.tool ?? ''} setup={setup} onChange={(tool) => edit(i, { tool })} />
            <label class="field">Model
              <input value={r.model ?? ''} placeholder="the tool's default" onInput={(e) => edit(i, { model: (e.target as HTMLInputElement).value })} />
            </label>
            <button type="button" disabled={reviewers.length < 2} title={reviewers.length < 2 ? 'A preset needs at least one reviewer' : undefined}
              onClick={() => setReviewers((rs) => rs.filter((_, n) => n !== i))}>Remove</button>
          </li>
        ))}
      </ul>
      <button type="button" disabled={profiles.length === 0}
        onClick={() => setReviewers((rs) => [...rs, { profile: profiles[0]?.name ?? '' }])}>Add a reviewer</button>
      <details class="advanced">
        <summary>More options</summary>
        <p class="muted">Left empty, the synthesizer runs on the first reviewer's tool and the verifier follows the synthesizer.</p>
        <div class="row">
          <ToolField label="Synthesizer on" value={synthTool} setup={setup} onChange={setSynthTool} />
          <label class="field">Synthesizer model
            <input value={synthModel} placeholder="the tool's default" onInput={(e) => setSynthModel((e.target as HTMLInputElement).value)} />
          </label>
        </div>
        <div class="row">
          <ToolField label="Verifier on" value={verifyTool} setup={setup} onChange={setVerifyTool} />
          <label class="field">Verifier model
            <input value={verifyModel} placeholder="the tool's default" onInput={(e) => setVerifyModel((e.target as HTMLInputElement).value)} />
          </label>
        </div>
      </details>
    </EditForm>
  )
}

// ToolField picks an agent tool, or leaves it to vuoro. An empty value means
// whichever tool is available, which is what most presets want.
function ToolField({ label, value, setup, onChange }: {
  label: string; value: string; setup: Setup; onChange: (tool: string) => void
}) {
  return (
    <label class="field">{label}
      <select value={value} onChange={(e) => onChange((e.target as HTMLSelectElement).value)}>
        <option value="">Any tool available</option>
        {setup.tools.map((t) => <option value={t.name} key={t.name}>{t.tool}</option>)}
      </select>
    </label>
  )
}

// Presets lists every preset a run can start from, the person's own first
// and then the ones vuoro ships (D61).
export function Presets({ setup, onSetup }: { setup: Setup; onSetup: (s: Setup) => void }) {
  // editing holds the preset being written and the name it replaces. A copy
  // has no original, so it saves as a new one.
  const [editing, setEditing] = useState<{ preset: Preset; original?: string }>()
  const [removing, setRemoving] = useState<string>()
  const { busy, error, run } = useAction()
  const names = setup.presets.map((p) => p.name)
  const profiles = setup.profiles ?? []

  const remove = (name: string) => run(async () => {
    onSetup(await post<Setup>('/api/setup/presets/remove', { base: setup.base, name }))
    setRemoving(undefined)
  })
  const saved = (s: Setup) => { setEditing(undefined); onSetup(s) }

  return (
    <div class="presets">
      <ul class="preset-cards">
        {setup.presets.map((p) => (
          <li key={p.name} class="preset-card">
            {editing?.original === p.name ? (
              <PresetEditor setup={setup} initial={editing.preset} original={p.name} onSaved={saved} onCancel={() => setEditing(undefined)} />
            ) : (
              <div class="preset-facts">
                <h3>{p.name}{p.builtin && <span class="muted"> · shipped with vuoro</span>}</h3>
                <p class="muted">{describe(p)}</p>
                {setup.editable && (
                  <p class="preset-actions">
                    {p.builtin ? (
                      <button type="button" onClick={() => setEditing({ preset: { ...p, name: copyName(p.name, names), builtin: false } })}>Copy</button>
                    ) : (
                      <>
                        <button type="button" onClick={() => setEditing({ preset: p, original: p.name })}>Edit</button>{' '}
                        {removing === p.name ? (
                          <>Remove {p.name}? <button type="button" disabled={busy} onClick={() => void remove(p.name)}>{busy ? 'Removing…' : 'Remove'}</button>{' '}
                            <button type="button" disabled={busy} onClick={() => setRemoving(undefined)}>Keep</button></>
                        ) : <button type="button" onClick={() => setRemoving(p.name)}>Remove</button>}
                      </>
                    )}
                  </p>
                )}
              </div>
            )}
          </li>
        ))}
      </ul>
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      {editing && editing.original === undefined && (
        <PresetEditor setup={setup} initial={editing.preset} onSaved={saved} onCancel={() => setEditing(undefined)} />
      )}
      {setup.editable && !editing && (
        <button type="button" disabled={profiles.length === 0}
          onClick={() => setEditing({ preset: { name: '', reviewers: [{ profile: profiles[0]?.name ?? '' }] } })}>Add a preset</button>
      )}
      {setup.config_path && <p class="muted">Your own presets are saved in <span class="mono">{setup.config_path}</span>, which you can also edit by hand. The ones vuoro ships are not in the file.</p>}
      {!setup.editable && setup.config_error && <p class="warn">Presets cannot save until the file loads again. Fix it by hand, then open Settings again.</p>}
    </div>
  )
}
