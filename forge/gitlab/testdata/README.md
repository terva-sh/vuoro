# GitLab fixtures

Recorded on 2026-10-02 from anonymous GraphQL (`POST https://gitlab.com/api/graphql`)
against two public merge requests in gitlab-org/cli. `mr3958.query.graphql` is
the query, with `iid` 3958. The 3991 file used the same query with `iid` 3991.

| File | Merge request | Why |
|---|---|---|
| `mr3958.json` | [gitlab-org/cli!3958](https://gitlab.com/gitlab-org/cli/-/merge_requests/3958), merged | six diff versions, two "requested changes", a re-requested review, two approvals, resolved diff threads |
| `mr3991.json` | [gitlab-org/cli!3991](https://gitlab.com/gitlab-org/cli/-/merge_requests/3991), open | changes requested, then the author pushed and replied: the reviewer's turn, with unresolved threads |

Scrubbed before commit: the body of every note that is not a system note is
replaced with `<note body removed>`. The turn rules read only system notes'
text. The query asks for usernames only, so there are no emails. There is no
token in any request.
