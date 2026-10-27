`pr13899.json` and `pr14578.json` are the adapter's own read query, answered by
GitHub for two open pull requests on the public repository cli/cli, #13899 and
#14578, on 2026-10-08. `files*.json` and `compare*.json` are the REST files list
and comparison for each. They were read through `gh api` as the owner's account.

They were scrubbed before they were committed:

- Accounts are replaced: the pull request's author is `author`, and other
  people are `reviewer1` and so on, in order of their first review. A bot is
  `review-bot1` and so on, and keeps its `Bot` type.
- Every body is replaced with `Note N.`, the titles with `Example change N`,
  and the branches with `feature` and `trunk`.
- The repository is `owner/repo`, and its permission is set to READ for
  #13899 and WRITE for #14578, so both cases of CanMerge are covered.
- Only the first two files remain, with each patch cut to its first four
  lines, and `changedFiles` is set to match.
- Commit SHAs and times are kept, because the turn rules read them.

- `labels`, `files`, `mergeable`, `headRepository` and `commits`, which project
  triage added to the query on 2026-10-27, were written in by hand rather than
  recorded. Each file's counts come from its cut patch, #14578 carries the
  label `enhancement` and a passing check rollup, and #13899 has neither.

No headers, tokens or emails were recorded. The tests serve these as GitHub
Enterprise Server would, at the test server's host, and rewrite the
`github.com` URLs in them to that host.
