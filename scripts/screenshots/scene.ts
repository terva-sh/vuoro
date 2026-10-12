// The world the README screenshots show: one person, riley, reviewing on
// three forges, with one change opened, reviewed by agents, and fixed by the
// editor. Every name, host, and commit here is made up.
//
// The fixtures are typed by the frontend's own types, so `just
// web-typecheck` fails when the API moves away from them, and capture.ts
// answers each /api/ route from here. Times are hours before now, which the
// capture freezes, so every "3h ago" reads the same on each run.
import type { InboxRow, InboxView } from '../../web/src/inbox'
import type {
  Change, Delivery, Draft, EditorDetail, FileDiff, LocalView, Run, Session, Setup, Usage,
} from '../../web/src/types'

export const now = Date.parse('2026-10-13T15:00:00Z')

const at = (hoursAgo: number) => new Date(now - hoursAgo * 3_600_000).toISOString()

export const version = '0.4.0'

const me = 'riley'

export function setup(scheme: Setup['scheme']): Setup {
  return {
    editable: true,
    base: 'screenshots',
    forges: [
      { name: 'Work GitLab', kind: 'gitlab', url: 'https://gitlab.example.com', username: me, token_command: ['secret-tool', 'lookup', 'vuoro', 'gitlab.example.com'] },
      { name: 'Forgejo', kind: 'forgejo', url: 'https://git.example.org', username: me, token_command: ['secret-tool', 'lookup', 'vuoro', 'git.example.org'] },
      { name: 'GitHub', kind: 'github', url: 'https://github.com', username: me, token_env: 'GITHUB_TOKEN', env_set: true },
    ],
    sources: [],
    tools: [{ name: 'claude', tool: 'Claude Code', version: '2.4.0', account: 'riley@example.com', state: 'ok' }],
    presets: [{ name: 'thorough', reviewers: [{ profile: 'code' }, { profile: 'security' }, { profile: 'documentation' }] }],
    profiles: [],
    waived: [],
    scheme,
    tab: 'reviews',
    background: {
      refresh_minutes: 10, notify: true, autostart: false, login: true,
      dock: true, dock_here: false, dock_restart: false,
    },
  }
}

export const hello = { version }

// The Local tab's badge reads the last view it built.
export const localCached: LocalView = {
  roots: [], added: [], scanned: at(1), built: at(1), projects: [], hidden: [], unreadable: [], needs: 2, quiet_days: 14,
}

// The inbox.

const accounts: InboxView['accounts'] = [
  { key: 'gitlab', name: 'Work GitLab', kind: 'gitlab', host: 'gitlab.example.com', username: me, last_attempt: at(0.05), last_success: at(0.05) },
  { key: 'forgejo', name: 'Forgejo', kind: 'forgejo', host: 'git.example.org', username: me, last_attempt: at(0.05), last_success: at(0.05) },
  { key: 'github', name: 'GitHub', kind: 'github', host: 'github.com', username: me, last_attempt: at(0.05), last_success: at(0.05) },
]

function row(r: {
  host: string; kind: string; account: string; project: string; number: number; title: string
  roles: string[]; owner: 'mine' | 'theirs'; lane?: InboxRow['lane']; since: number; reasons: [string, string][]
  summary?: string[]; waiting?: InboxRow['waiting']; draft?: boolean; quiet?: number; snoozed?: string
}): InboxRow {
  const path = r.kind === 'gitlab' ? `-/merge_requests/${r.number}` : `pulls/${r.number}`
  return {
    ref: { host: r.host, project: r.project, number: r.number },
    kind: r.kind, account_key: r.kind, account: r.account, title: r.title,
    web_url: `https://${r.host}/${r.project}/${path}`, draft: !!r.draft, roles: r.roles,
    turn: {
      owner: r.owner, since: at(r.since), last_activity: at(r.since),
      reasons: r.reasons.map(([rule, text]) => ({ rule, text, since: at(r.since) })),
    },
    lane: r.lane, summary: r.summary ?? r.reasons.map(([, text]) => text), waiting: r.waiting,
    quiet_since: r.quiet === undefined ? at(r.since) : at(r.quiet),
    snoozed_until: r.snoozed,
  }
}

const gitlab = { host: 'gitlab.example.com', kind: 'gitlab', account: 'Work GitLab' }
const forgejo = { host: 'git.example.org', kind: 'forgejo', account: 'Forgejo' }
const github = { host: 'github.com', kind: 'github', account: 'GitHub' }

export const inbox: InboxView = {
  accounts,
  mine: [
    row({
      ...gitlab, project: 'harbor/billing-api', number: 405, title: 'Retry webhook deliveries with exponential backoff',
      roles: ['author'], owner: 'mine', lane: 'yours', since: 26,
      reasons: [['A2', 'mika requested changes; no new commits have answered them.']],
    }),
    row({
      ...github, project: 'example/fieldnotes', number: 88, title: 'Sync notes in the background on battery',
      roles: ['author'], owner: 'mine', lane: 'yours', since: 5,
      reasons: [['A3', 'Approved and mergeable; you can merge this change.']],
    }),
    row({
      ...forgejo, project: 'tools/ledger', number: 131, title: 'Explain the cache eviction order',
      roles: ['author'], owner: 'mine', lane: 'replies', since: 19,
      reasons: [['A1', 'jun is waiting for your answer in an unresolved thread.'], ['A1', 'jun is waiting for your answer in an unresolved thread.']],
      summary: ['2 threads await your answer: jun (2).'],
      waiting: [
        { rule: 'A1', who: 'jun', where: 'cache/lru.go:42', since: at(19), text: 'Does eviction still run when the clock goes backwards?' },
        { rule: 'A1', who: 'jun', where: 'README.md:118', since: at(20), text: 'Worth saying here that the order is by last read, not last write.' },
      ],
    }),
    row({
      ...gitlab, project: 'harbor/billing-api', number: 412, title: 'Stream invoice exports instead of buffering them',
      roles: ['reviewer'], owner: 'mine', lane: 'reviews', since: 3,
      reasons: [['R1', 'Your review is requested.']],
    }),
    row({
      ...forgejo, project: 'tools/ledger', number: 129, title: 'Add a dry-run flag to the importer',
      roles: ['reviewer'], owner: 'mine', lane: 'reviews', since: 50,
      reasons: [['R2', 'New commits arrived since your review.']],
    }),
    row({
      ...github, project: 'example/fieldnotes', number: 91, title: 'Handle leap seconds in the timeline',
      roles: ['reviewer'], owner: 'mine', lane: 'reviews', since: 400,
      reasons: [['R1', 'Your review is requested.']],
    }),
  ],
  theirs: [
    row({
      ...gitlab, project: 'harbor/web', number: 233, title: 'Move the settings page to the new form components',
      roles: ['author'], owner: 'theirs', since: 30, reasons: [['waiting', 'Waiting for mika; no unanswered review action is recorded for you.']],
    }),
    row({
      ...forgejo, project: 'tools/ledger', number: 127, title: 'Read amounts as decimals, not floats',
      roles: ['reviewer'], owner: 'theirs', since: 70, reasons: [['waiting', 'Waiting for jun; no unanswered review action is recorded for you.']],
    }),
  ],
  dormant: [
    row({
      ...github, project: 'example/fieldnotes', number: 64, title: 'Export a notebook as a static site', draft: true,
      roles: ['author'], owner: 'theirs', since: 900, quiet: 900, reasons: [['waiting', 'Waiting for a reviewer.']],
    }),
  ],
  snoozed: [
    row({
      ...gitlab, project: 'harbor/billing-api', number: 398, title: 'Bump the Postgres driver',
      roles: ['reviewer'], owner: 'mine', since: 80, snoozed: '9999-12-31T00:00:00Z', reasons: [['R1', 'Your review is requested.']],
    }),
  ],
}

// The change under review: harbor/billing-api!412.

const head = '7c41e09a5d2b8f3361a0c4e9b17d52f8e6a3c0d1'
const base = '2f9a6c1e8b4d07a35e9c12f6d8b0a4e7c3d5f912'
const diff = { base, start: base, head }
const url = 'https://gitlab.example.com/harbor/billing-api/-/merge_requests/412'

const files: FileDiff[] = [
  {
    old_path: 'export/invoices.go', new_path: 'export/invoices.go', new: false, deleted: false, renamed: false, rebuilt: false, omitted: false,
    patch: `@@ -58,22 +58,30 @@ func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
 	from, to, err := period(r.URL.Query())
 	if err != nil {
 		http.Error(w, err.Error(), http.StatusBadRequest)
 		return
 	}
-	invoices, err := h.store.Invoices(r.Context(), from, to)
+	rows, err := h.store.InvoiceRows(r.Context(), from, to)
 	if err != nil {
 		http.Error(w, "reading invoices", http.StatusInternalServerError)
 		return
 	}
-	var buf bytes.Buffer
-	if err := writeCSV(&buf, invoices); err != nil {
-		http.Error(w, "writing invoices", http.StatusInternalServerError)
-		return
-	}
 	w.Header().Set("Content-Type", "text/csv")
-	w.Write(buf.Bytes())
+	out := newWriter(w, 500)
+	for rows.Next() {
+		var inv Invoice
+		if err := rows.Scan(&inv); err != nil {
+			log.Printf("export: %v", err)
+			return
+		}
+		if err := out.Write(inv); err != nil {
+			return
+		}
+	}
+	out.Flush()
 }
`,
  },
  {
    old_path: 'export/writer.go', new_path: 'export/writer.go', new: true, deleted: false, renamed: false, rebuilt: false, omitted: false,
    patch: `@@ -0,0 +1,52 @@
+package export
+
+import (
+	"encoding/csv"
+	"io"
+	"net/http"
+)
+
+// writer writes invoices as CSV rows, flushing every batch rows so a large
+// export reaches the client as it is read.
+type writer struct {
+	csv    *csv.Writer
+	flush  http.Flusher
+	batch  int
+	n      int
+	header bool
+}
+
+func newWriter(w io.Writer, batch int) *writer {
+	f, _ := w.(http.Flusher)
+	return &writer{csv: csv.NewWriter(w), flush: f, batch: batch}
+}
+
+func (w *writer) Write(inv Invoice) error {
+	if !w.header {
+		if err := w.csv.Write(columns); err != nil {
+			return err
+		}
+		w.header = true
+	}
+	if err := w.csv.Write(inv.Record()); err != nil {
+		return err
+	}
+	w.n++
+	if w.n%w.batch == 0 {
+		w.Flush()
+	}
+	return w.csv.Error()
+}
+
+func (w *writer) Flush() {
+	w.csv.Flush()
+	if w.flush != nil {
+		w.flush.Flush()
+	}
+}
`,
  },
  {
    old_path: 'export/invoices_test.go', new_path: 'export/invoices_test.go', new: false, deleted: false, renamed: false, rebuilt: false, omitted: false,
    patch: `@@ -21,6 +21,21 @@ func TestExportWritesEveryInvoice(t *testing.T) {
 	}
 }

+func TestExportStreamsInBatches(t *testing.T) {
+	store := fakeStore(1200)
+	rec := httptest.NewRecorder()
+	h := &Handler{store: store}
+	h.ServeHTTP(rec, httptest.NewRequest("GET", "/export?from=2026-09-01&to=2026-10-01", nil))
+	if rec.Code != http.StatusOK {
+		t.Fatalf("status %d", rec.Code)
+	}
+	lines := strings.Count(rec.Body.String(), "\\n")
+	if lines != 1201 {
+		t.Fatalf("got %d lines, want a header and 1200 rows", lines)
+	}
+}
+
 func TestExportRefusesABadPeriod(t *testing.T) {
 	rec := httptest.NewRecorder()
 	h := &Handler{store: fakeStore(0)}
`,
  },
]

export const change: Change = {
  kind: 'gitlab',
  capabilities: { drafts: true, reply: true, resolve: true, approve: true, unapprove: true, request_changes: true, suggestions: true },
  ref: { host: 'gitlab.example.com', project: 'harbor/billing-api', number: 412 },
  title: 'Stream invoice exports instead of buffering them',
  description: 'A year of invoices no longer fits in memory on the smaller workers. This reads the rows with a cursor and writes CSV as it goes, flushing every 500 rows.\n\nCloses #377.',
  author: { username: 'sam', bot: false }, web_url: url,
  source_branch: 'sam/stream-exports', target_branch: 'main', diff,
  draft: false, state: 'open', mergeability: 'MERGEABLE',
  created: at(9), updated: at(3),
  reviewers: [
    { username: me, bot: false, state: 'unreviewed', can_merge: true },
    { username: 'mika', bot: false, state: 'approved', can_merge: true },
  ],
  assignees: [{ username: 'sam', bot: false }], participants: null,
  requests: [{ reviewer: me, by: 'sam', at: at(3) }], reviews: null,
  pushes: [{ at: at(3.2), head, by: 'sam' }],
  threads: [], files,
  // The two findings riley kept, as their own pending comments.
  drafts: [
    { id: '9001', body: '**The CSV writer is not flushed when a row fails**', thread_id: '', anchor: { path: 'export/invoices.go', old_path: 'export/invoices.go', new_line: 76, old_line: 0, diff } },
    { id: '9002', body: '**A cancelled request leaves the cursor open**', thread_id: '', anchor: { path: 'export/invoices.go', old_path: 'export/invoices.go', new_line: 64, old_line: 0, diff } },
  ],
}

// The run: three reviewers, a synthesizer, a verifier, and an editor.

const usage = (input: number, output: number, cost: number): Usage => ({ input, output, cache_read: input * 3, cache_write: input / 2, cost_usd: cost })

function session(id: string, profile: string, role: Session['role'], u: Usage): Session {
  return {
    id, profile, role, tool: 'claude', model: 'claude-opus-5-5', status: 'ended', usage: u, log: [],
    info: { tool: 'Claude Code', version: '2.4.0', model: 'claude-opus-5-5', account: 'riley@example.com', tools: [] },
  }
}

const sessions: Session[] = [
  session('s1', 'code', 'reviewer', usage(41_000, 3_900, 0.62)),
  session('s2', 'security', 'reviewer', usage(38_500, 2_700, 0.51)),
  session('s3', 'documentation', 'reviewer', usage(29_000, 1_600, 0.34)),
  session('s4', 'synthesis', 'synthesizer', usage(18_000, 2_200, 0.29)),
  session('s5', 'verification', 'verifier', usage(52_000, 3_100, 0.71)),
  { ...session('s6', 'editor', 'editor', usage(64_000, 5_400, 0.94)), status: 'idle' },
]

const drafted = 'Claude Code · claude-opus-5-5'

function draft(d: Partial<Draft> & Pick<Draft, 'id' | 'severity' | 'title' | 'description' | 'session'>): Draft {
  return { status: 'pending', revision: 1, drafted_by: drafted, updated: at(2), ...d }
}

const confirmed = (reason: string) => ({ level: 'confirmed' as const, reason, by: 'verification', at: at(1.6) })

// What each reviewer drafted, which the synthesizer's findings cite.
const raised: Draft[] = [
  draft({ id: 'd1', session: 's1', severity: 'high', title: 'Writer is not flushed on the error path', description: 'A failed Write returns before Flush.', anchor: { path: 'export/invoices.go', side: 'head', start_line: 76, end_line: 77 } }),
  draft({ id: 'd2', session: 's2', severity: 'high', title: 'Buffered rows are dropped when a write fails', description: 'Rows written since the last flush never reach the client.', anchor: { path: 'export/invoices.go', side: 'head', start_line: 76, end_line: 77 } }),
  draft({ id: 'd3', session: 's2', severity: 'high', title: 'rows is never closed', description: 'No defer rows.Close().', anchor: { path: 'export/invoices.go', side: 'head', start_line: 64, end_line: 64 } }),
  draft({ id: 'd4', session: 's1', severity: 'medium', title: 'rows.Err is not checked', description: 'A cursor that stops on an error looks like the end of the export.', anchor: { path: 'export/invoices.go', side: 'head', start_line: 72, end_line: 72 } }),
  draft({ id: 'd5', session: 's3', severity: 'low', title: 'The batch size is a bare number', description: 'Name it, and say why 500.', anchor: { path: 'export/invoices.go', side: 'head', start_line: 70, end_line: 70 } }),
  draft({ id: 'd6', session: 's1', severity: 'info', title: 'No test for an empty period', description: 'An export with no rows writes nothing at all, not even the header.', anchor: { path: 'export/invoices_test.go', side: 'head', start_line: 24, end_line: 24 } }),
]

const findings: Draft[] = [
  draft({
    id: 'd7', session: 's4', severity: 'high', sources: ['d1', 'd2'],
    title: 'The CSV writer is not flushed when a row fails',
    description: 'When `out.Write` fails the handler returns, and up to 499 rows already written sit in the `csv.Writer`\'s buffer. The client gets a truncated file with a `200 OK`, and no sign that rows are missing.',
    evidence: 'if err := out.Write(inv); err != nil {\n\treturn\n}',
    anchor: { path: 'export/invoices.go', side: 'head', start_line: 76, end_line: 77 },
    confidence: confirmed('Traced with a store that fails on row 3: the recorder got the header and two rows, and no error.'),
    decision: 'kept', note_id: '9001',
  }),
  draft({
    id: 'd8', session: 's4', severity: 'high', sources: ['d3'],
    title: 'A cancelled request leaves the cursor open',
    description: '`rows` is never closed. A client that disconnects mid-export leaves the cursor, and its connection, held until the pool times it out.',
    anchor: { path: 'export/invoices.go', side: 'head', start_line: 64, end_line: 64 },
    confidence: confirmed('InvoiceRows returns *sql.Rows, and nothing on any path calls Close.'),
    decision: 'kept', note_id: '9002',
  }),
  draft({
    id: 'd9', session: 's4', severity: 'medium', sources: ['d4'],
    title: 'An error from the cursor reads as the end of the export',
    description: '`rows.Next` returns false on an error as well as at the end. Without a check of `rows.Err()` after the loop, a dropped connection produces a short file that looks complete.',
    anchor: { path: 'export/invoices.go', side: 'head', start_line: 72, end_line: 72 },
    confidence: { level: 'plausible', reason: 'The driver does surface network errors through Err; not reproduced against a real database.', by: 'verification', at: at(1.6) },
  }),
  draft({
    id: 'd10', session: 's4', severity: 'low', sources: ['d5'],
    title: 'The batch size is a bare number',
    description: 'Name the 500, and say in a comment why it was chosen.',
    anchor: { path: 'export/invoices.go', side: 'head', start_line: 70, end_line: 70 },
    confidence: { level: 'doubtful', reason: 'Style only; the description of the change already explains the number.', by: 'verification', at: at(1.6) },
    decision: 'dropped',
  }),
  draft({
    id: 'd11', session: 's4', severity: 'info', sources: ['d6'],
    title: 'No test covers an empty period',
    description: 'With no rows the header is never written, so the client gets an empty body rather than a CSV with only its header. A test would pin down which is intended.',
    anchor: { path: 'export/invoices_test.go', side: 'head', start_line: 24, end_line: 24 },
    confidence: confirmed('An empty fakeStore produces a zero-length body.'),
  }),
]

export const run: Run = {
  id: 'r-20261013-412', kind: 'gitlab', ref: change.ref, title: change.title, web_url: url, head, base,
  created: at(2.4), status: 'idle', cost_usd: 3.41,
  sessions, drafts: [...raised, ...findings],
  editor: { session: 's6', branch: 'vuoro/harbor-billing-api-412', base: head, started: at(0.9), deliver: 'suggestions' },
}

// An earlier run of another change, for the home page's list of runs.
const earlier: Run = {
  id: 'r-20261012-129', kind: 'forgejo', ref: { host: 'git.example.org', project: 'tools/ledger', number: 129 },
  title: 'Add a dry-run flag to the importer', web_url: 'https://git.example.org/tools/ledger/pulls/129',
  head: 'e1d0c9b8a7f6e5d4c3b2a1f0e9d8c7b6a5f4e3d2', base: 'a5f4e3d2c1b0a9f8e7d6c5b4a3f2e1d0c9b8a7f6',
  created: at(52), status: 'closed', cost_usd: 1.87, sessions: [], drafts: [],
}

export const runs: Run[] = [run, earlier]

export const delivery: Delivery = { offered: true, deliver: 'suggestions' }

// The editor's two commits, each naming the finding it answers.
export const editor: EditorDetail = {
  ...run.editor!,
  commits: [
    { sha: 'b3e8d2f61c9a47e05d1f8a2c6e4b9d7038a51c2e', subject: 'Flush the export before returning on a failed row', findings: ['d7'] },
    { sha: '4f7a1c0e9d2b8365a7e1f0c4d9b2e6a8173c5f0b', subject: 'Close the invoice cursor when the export ends', findings: ['d8'] },
  ],
  files: [{
    old_path: 'export/invoices.go', new_path: 'export/invoices.go', new: false, deleted: false, renamed: false, rebuilt: false, omitted: false,
    patch: `@@ -66,6 +66,7 @@ func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
 		http.Error(w, "reading invoices", http.StatusInternalServerError)
 		return
 	}
+	defer rows.Close()
 	w.Header().Set("Content-Type", "text/csv")
 	out := newWriter(w, 500)
 	for rows.Next() {
@@ -75,6 +76,7 @@ func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
 			return
 		}
 		if err := out.Write(inv); err != nil {
+			out.Flush()
 			return
 		}
 	}
`,
  }],
}
