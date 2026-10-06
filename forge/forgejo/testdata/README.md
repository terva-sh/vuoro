The PR metadata, the README/decision-log file inventory and diff, and the push
and comment timeline come from terva-sh/vuoro PR #2 on the internal Forgejo,
read on 2026-10-02 through `tea api`. Only fields the adapter consumes remain.
Hosts and accounts are replaced, descriptions and the review summary are
shortened, and unrelated files and timeline events are omitted. No headers,
credentials, emails or authentication files were recorded.

The additional review/thread cases in forgejo_test.go are synthetic API-shaped
responses checked against this instance's swagger.v1.json. They cover states
absent from the merged PR. Forgejo's `position` is the new-side line and
`original_position` is the old-side line; they are not diff-row offsets.

`writes/spike.json` records the Forgejo write spike,
TKT-01M47EG9BHW0HBVFBTSBB1915X. It was read on 2026-10-06 from a private sandbox
PR on the internal Forgejo (15.0.3) through `tea api`, as two accounts. Each
case holds the request, the status, the response trimmed to the fields an
adapter reads, and a note saying what it shows. The accounts are `author` and
`reviewer`, the repository is `author/sandbox`, and the host is
`forgejo.example`. No headers, credentials or emails were recorded. docs/plan.md
section 5.3 summarizes it.
