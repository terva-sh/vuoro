// The API's JSON, as forge/forge.go and review/view.go write it.

export interface Ref { host: string; project: string; number: number }
export interface User { username: string; bot: boolean }
export interface DiffRefs { base: string; start: string; head: string }

export interface Reviewer extends User {
  state: 'unreviewed' | 'reviewed' | 'requested_changes' | 'approved'
  can_merge: boolean
}
export interface Review { reviewer: string; verdict: 'approved' | 'changes_requested' | 'commented'; at: string; sha: string; dismissed: boolean; stale: boolean }
export interface ReviewRequest { reviewer: string; by: string; at: string }
export interface Push { at: string; head: string; by: string }

export interface Anchor { path: string; old_path: string; new_line: number; old_line: number; diff: DiffRefs }
export interface Note { id: string; author: User; body: string; at: string; system: boolean }
export interface Thread { id: string; anchor: Anchor | null; resolvable: boolean; resolved: boolean; notes: Note[] }

export interface FileDiff {
  old_path: string; new_path: string
  new: boolean; deleted: boolean; renamed: boolean
  patch: string; rebuilt: boolean; omitted: boolean
}

export interface Capabilities { drafts: boolean; resolve: boolean; approve: boolean }

export interface Change {
 kind: string; capabilities: Capabilities
  ref: Ref; title: string; description: string; author: User; web_url: string
  source_branch: string; target_branch: string; diff: DiffRefs
  draft: boolean; state: 'open' | 'merged' | 'closed'; mergeability: string
  created: string; updated: string
  reviewers: Reviewer[] | null; assignees: User[] | null; participants: User[] | null
  requests: ReviewRequest[] | null; reviews: Review[] | null; pushes: Push[] | null
  threads: Thread[] | null; files: FileDiff[] | null
  drafts: ForgeDraft[] | null
}

// ForgeDraft is one of the person's own pending comments on the forge.
export interface ForgeDraft { id: string; body: string; thread_id: string; anchor: Anchor | null }

export type Severity = 'critical' | 'high' | 'medium' | 'low' | 'info'

export interface DraftAnchor { path: string; side: 'head' | 'base'; start_line: number; end_line: number }
export type TriageCategory = "must_fix" | "discuss" | "nit"
export interface Draft {
  thread_id?: string; category?: TriageCategory; reason?: string; thread_key?: string; fixed_in?: string
  id: string; severity: Severity; title: string; description: string; evidence?: string
  anchor?: DraftAnchor; sources?: string[]
  suggestion?: string // replaces the anchor's head-side lines
  status: 'pending' | 'withdrawn'; revision: number; session: string; drafted_by: string; updated: string
  decision?: 'kept' | 'dropped'; note_id?: string; edited?: boolean
  confidence?: Confidence // the verifier's mark
  override?: Confidence // the person's, which wins
}

export type Level = 'confirmed' | 'plausible' | 'doubtful'
export interface Confidence { level: Level; reason?: string; by: string; at: string }

export type Status = 'preparing' | 'working' | 'idle' | 'ended' | 'failed' | 'closed' | 'waiting' | 'cancelled'

export interface Entry {
  kind: 'change' | 'user' | 'assistant' | 'tool' | 'denied' | 'error'
  text?: string; tool?: string; input?: string; output?: string; is_error?: boolean; at: string
}
export interface Usage { input: number; output: number; cache_read: number; cache_write: number; cost_usd: number }
export interface SessionInfo { tool: string; version: string; model: string; account: string; tools: string[] }
export interface Session {
  id: string; profile: string; tool: string; model?: string; role: 'reviewer' | 'synthesizer' | 'verifier' | 'triage' | 'summary' | 'suggester' | 'editor'
  target?: string // a suggester's draft
  status: Status; error?: string; info?: SessionInfo; usage: Usage; log: Entry[] | null
}
export interface Run {
  mode?: "triage" | "summary"; triage_threads?: Thread[]
  review_complete?: boolean; summary?: { scope: "change" | "since_review"; from: string; to: string; review_id?: string }
  id: string; kind: string; ref: Ref; title: string; web_url: string; head: string; base: string
  created: string; status: Status; error?: string; cost_usd: number
  sessions: Session[]; drafts: Draft[]
  editor?: EditorState
}

// EditorState is a run's editor, which fixes the kept findings on a local
// branch (D39).
export interface EditorState {
  session: string; branch: string; base: string; started: string; discarded?: boolean
  own?: boolean; items?: string[] // a fix of the person's own change, and the drafts it was given (D41, D42)
  local?: boolean // a local range's fix, handed back as a branch in the person's repository (D42)
  pushed?: { branch: string; sha: string; target: string; url?: string; at: string }
}
// StackPreview is where a push of the editor's work would go, before the
// person confirms it.
export interface StackPreview {
  branch: string; target: string; sha: string; commits: number; files: string[]; ci?: string[]
  moved?: boolean; own?: boolean; local?: boolean; title?: string; description?: string; opened?: string
}
export interface EditorCommit { sha: string; subject: string; findings: string[] }
export interface EditorDetail extends EditorState { commits: EditorCommit[]; files: FileDiff[] }

export interface Preset { name: string; reviewers: { profile: string; tool?: string; model?: string }[] }
export interface Setup {
  config_error?: string
  forges: { name: string; url: string; username: string }[]
  tools: { name: string; tool: string; version?: string; account?: string; error?: string }[]
  presets: Preset[]
  profiles: { name: string; title: string; prompt: string }[]
  waived: string[] // confirmations the person turned off
}

export const refKey = (r: Ref) => `${r.host}/${r.project}!${r.number}`

export const changeNumber = (kind: string | undefined, n: number) => `${kind === 'forgejo' ? '#' : '!'}${n}`
