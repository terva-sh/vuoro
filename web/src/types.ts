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

// Capabilities are the writes this forge, with these credentials, can do.
// The UI offers only these; the server refuses the rest.
export interface Capabilities {
  drafts: boolean; reply: boolean; resolve: boolean; approve: boolean; unapprove: boolean; request_changes: boolean
  // suggestions: the forge applies a suggestion block. Without it a suggestion posts as a plain replacement.
  suggestions: boolean
  // span_suggestions: a suggestion replaces the lines its comment spans, as a bare block (GitHub).
  span_suggestions?: boolean
}

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
  fixed_subject?: string; fixed_by?: 'editor' | 'person' | 'suggestion' // the fixing commit's subject, and who linked it (D50, D59)
  suggested?: SuggestedFix // the editor's fix, sent back as suggestions on the comments (D59)
  id: string; severity: Severity; title: string; description: string; evidence?: string
  anchor?: DraftAnchor; sources?: string[]
  suggestion?: string // replaces the anchor's head-side lines
  status: 'pending' | 'withdrawn'; revision: number; session: string; drafted_by: string; updated: string
  decision?: 'kept' | 'dropped'; note_id?: string; edited?: boolean
  earlier?: string; carried?: boolean // the earlier finding it restates, and whether its decision came from there (D50)
  confidence?: Confidence // the verifier's mark
  override?: Confidence // the person's, which wins
  change?: number; reviewers?: string[]; labels?: string[]; unblock?: string // a project triage proposal (D65)
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
  mode?: "triage" | "summary" | "project-triage" | "update"; triage_threads?: Thread[]
  update?: Update
  project_triage?: { path: string; changes: { number: number; stamp: string }[]; candidates: string[]; labels: string[] }
  review_complete?: boolean; summary?: { scope: "change" | "since_review"; from: string; to: string; review_id?: string }
  id: string; kind: string; ref: Ref; title: string; web_url: string; head: string; base: string
  created: string; status: Status; error?: string; cost_usd: number
  sessions: Session[]; drafts: Draft[]
  editor?: EditorState
  waves?: EditorState[] // the editors before this one, each pushed (D50)
  follows?: string; followed_by?: string[] // the review this one follows, and the ones following it (D50)
  earlier?: Earlier[] // the findings of the review it follows
  patches?: Patch[] // the fixes it saved as patches (D51)
}
// Patch is a fix's commit, saved beside the run's record so its diff outlives
// the branch (D51). One over the size cap keeps only the files it touched.
export interface Patch { sha: string; subject: string; by: 'editor' | 'person'; too_large?: boolean; files?: string[]; at: string }
export interface PatchDetail extends Patch { patch?: string; diff: FileDiff[] }
// Earlier is a finding of the review a run follows, with where it stood when
// the run started (D50).
export interface Earlier {
  id: string; severity: Severity; title: string; anchor?: DraftAnchor
  standing: 'fixed' | 'dismissed' | 'kept' | 'open'; fixed_in?: string; fixed_subject?: string; decision?: 'kept' | 'dropped'
}

// Update is a change brought up to date with its target (section 6.10, D66,
// D67): vuoro merges or rebases, and an editor resolves each conflicting stop.
export interface Update {
  strategy: 'merge' | 'rebase'; source: string; target: string; target_sha: string; own: boolean; tool: string; model?: string
  onto?: string; remaining?: string[]; stops: Stop[]; result?: string; pushed?: Pushed; discarded?: boolean; dropped?: string[]
  started: string; finished?: string
}
export interface Stop {
  commit?: string; subject: string; seed: string; onto: string; conflicted: string[]; session?: string; branch?: string
  tip?: string; resolved?: string; outside?: string[]; accepted?: string
}
export interface UpdateDetail extends Update {
  state: '' | 'resolving' | 'ready' | 'pushed' | 'discarded'
  resolutions: FileDiff[][]; commits: { sha: string; subject: string }[]
}

// EditorState is a run's editor, which fixes the kept findings on a local
// branch (D39).
export interface EditorState {
  session: string; branch: string; base: string; started: string; discarded?: boolean
  own?: boolean; items?: string[] // a fix of the person's own change, and the drafts it was given (D41, D42)
  local?: boolean // a local range's fix, handed back as a branch in the person's repository (D42)
  pushed?: Pushed
  continues?: Pushed // a later wave's starting point, the last wave's push (D50)
  commits?: EditorCommit[] // an earlier wave's commits, as pushed
  deliver?: 'suggestions' // how the wave was started to go back: as suggestions, or as a draft change (D59)
  suggested?: { sha: string; at: string; comments: number } // the wave went back as suggestions
}
// SuggestedFix is an editor's commit sent back as suggestions (D59).
export interface SuggestedFix {
  commit: string; subject: string; at: string; misfits?: number
  hunks?: { path: string; start: number; end: number; text: string; note_id?: string }[]
}
// Delivery says whether the editor's fixes can go back as suggestions, and
// how they went back last on a change by the same author.
export interface Delivery { offered: boolean; deliver: string }
// Placed is one suggestion: lines start to end at the reviewed head become text.
export interface Placed { path: string; start: number; end: number; line: number; text: string; block: string }
export interface PlannedFix {
  finding: string; title: string; note_id: string; commits: EditorCommit[]
  on_comment?: Placed; comments?: Placed[]; misfits?: { path: string; reason: string }[]
  diff?: string; overflow?: { lines: number; files: number; bytes?: number; name: string; attach: boolean }
  done?: boolean // sent by an earlier attempt
}
// FixPreview is one finding's part of a suggestion delivery, as it will read.
export interface FixPreview extends PlannedFix {
  before?: string; body?: string; missing?: boolean
  new: { path: string; start: number; line: number; body: string }[]
}
export interface SuggestionPreview { sha: string; fixes: FixPreview[]; left?: string[] }
export interface Pushed { branch: string; sha: string; target: string; url?: string; not_draft?: boolean; at: string }
// StackPreview is where a push of the editor's work would go, before the
// person confirms it.
export interface StackPreview {
  branch: string; target: string; sha: string; commits: number; files: string[]; ci?: string[]
  moved?: boolean; own?: boolean; local?: boolean; title?: string; description?: string; opened?: string; not_draft?: boolean
}
export interface EditorCommit { sha: string; subject: string; findings: string[] }
export interface EditorDetail extends EditorState { commits: EditorCommit[]; files: FileDiff[] }

export interface PresetReviewer { profile: string; tool?: string; model?: string }
// PresetRole is a preset's synthesizer or verifier. They name a tool and a
// model only, because a preset ignores their profile. Leaving one out lets
// the run's own defaults apply (D11, D31).
export interface PresetRole { tool?: string; model?: string }
export interface Preset {
  name: string
  reviewers: PresetReviewer[]
  synthesizer?: PresetRole
  verifier?: PresetRole
  // builtin marks one vuoro ships, which the presets page copies rather than
  // edits, so the shipped set keeps its meaning across an upgrade (D61).
  builtin?: boolean
}
// ForgeForm is one forge as setup edits it. It names where the token comes
// from, never the token.
export interface ForgeForm {
  name: string; kind: string; url: string; username: string
  token_command?: string[]; token_env?: string; clone?: string
}
// Access is how far a token reaches, as its forge tells it (D57).
export type AccessLevel = 'unknown' | 'none' | 'read' | 'full' | 'broad'
export interface Access { level: AccessLevel; scopes?: string[]; broad?: string[]; missing?: string[]; note?: string }
// TokenSource is a place this machine can keep a token. {host} in read and
// store stands for the forge's host.
export interface TokenSource { id: string; label: string; read?: string[]; store?: string[]; note?: string }
export type ToolState = 'ok' | 'missing' | 'too_old' | 'signed_out' | 'broken'
export interface Tool {
  name: string; tool: string; version?: string; account?: string; error?: string; state: ToolState
  // shell_login says the tool found no login here while the person's login
  // shell holds one, which is what a Dock or start-at-login launch does. It
  // says only that a credential is set, never what it is.
  shell_login?: boolean
}
// HomeTab is a tab of the home page (docs/plan.md section 6.8); empty is the
// first, Reviews.
export type HomeTab = '' | 'reviews' | 'local' | 'projects'

// The Local tab: the person's checkouts, one row per branch beyond its base.
export interface LocalWorktree { path: string; branch?: string; head: string; dirty: boolean }
// LocalChange is the open change a local branch is pushed to (D49), and how
// the branch stands against its head.
export interface LocalChange {
  ref: Ref; kind: string; title: string; web_url: string; head: string; draft: boolean
  standing: 'in-step' | 'ahead' | 'behind' | 'diverged' | 'unfetched'; ahead: number; behind: number
}
export interface LocalBranch {
  name: string; tip: string; ahead: number; committed: string; worktrees: string[]
  upstream_remote?: string; upstream_branch?: string
  change?: LocalChange
  run?: { id: string; status: Status; head: string; created: string; moved: boolean; undecided: number }
  fixes: { branch: string; waiting: boolean; run: string; patches: Patch[] }[]
  needs: string[] // why it needs the person, empty when it does not
  quiet: boolean // no commit for the view's quiet_days, so unreviewed is not counted (D48)
}
export interface LocalProject {
  key: string; name: string; path: string; base?: string; error?: string
  remotes: { name: string; host: string; path: string }[]
  worktrees: LocalWorktree[]; branches: LocalBranch[]; added: boolean; base_chosen: boolean
}
export interface LocalView {
  roots: string[]; added: string[]; scanned: string; stopped?: string; built: string
  projects: LocalProject[]; hidden: LocalProject[]; unreadable: { path: string; error: string }[]
  needs: number
  quiet_days: number
}

// Scheme is a colour scheme. The empty one follows the system's.
export type Scheme = '' | 'light' | 'dark'

export interface Setup {
  config_error?: string
  config_path?: string
  editable: boolean
  base: string // the config file's hash as read; a save must match it
  forges: (ForgeForm & { env_set?: boolean })[]
  sources: TokenSource[]
  tools: Tool[]
  presets: Preset[]
  profiles: { name: string; title: string; prompt: string }[]
  waived: string[] // confirmations the person turned off
  scheme: Scheme // the pinned colour scheme, or empty for the system's
  tab: HomeTab // the home page's tab used last
  background: Background
}

// Background is how vuoro runs with its window closed (D55).
export interface Background {
  refresh_minutes: number
  notify: boolean
  autostart: boolean // vuoro starts at login
  login: boolean // this platform can start vuoro at login
  login_error?: string
  dock: boolean // vuoro keeps its macOS Dock icon, as saved (D58)
  dock_here: boolean // there is a Dock to choose
  dock_restart: boolean // the saved choice waits for vuoro to start again
}

export const refKey = (r: Ref) => `${r.host}/${r.project}!${r.number}`

// pullForge says the forge numbers pull requests with #, as Forgejo and GitHub
// do. A change of no kind is from before vuoro read any forge but GitLab.
const pullForge = (kind: string | undefined) => kind === 'forgejo' || kind === 'github'
export const changeNumber = (kind: string | undefined, n: number) => `${pullForge(kind) ? '#' : '!'}${n}`
// changeWord is what the forge calls a change.
export const changeWord = (kind: string | undefined) => (pullForge(kind) ? 'pull request' : 'merge request')
