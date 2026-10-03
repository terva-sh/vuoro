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
