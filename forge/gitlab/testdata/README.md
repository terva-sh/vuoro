# GitLab fixtures

Recorded on 2026-10-02 from anonymous GraphQL (`POST https://gitlab.com/api/graphql`)
against two public merge requests in gitlab-org/cli, with the adapter's own
query, `../query.graphql`, and no token.

| File | Merge request | Why |
|---|---|---|
| `mr3958.json` | [gitlab-org/cli!3958](https://gitlab.com/gitlab-org/cli/-/merge_requests/3958), merged | six diff versions, two "requested changes", a re-requested review, two approvals, resolved diff threads |
| `mr3991.json` | [gitlab-org/cli!3991](https://gitlab.com/gitlab-org/cli/-/merge_requests/3991), open | changes requested, then the author pushed and replied: the reviewer's turn, with unresolved threads |

Scrubbed before commit: the description, and the body of every note that is
not a system note, are replaced with placeholders. The turn rules read only system notes'
text. The query asks for usernames only, so there are no emails. There is no
token in any request.

`mr3991-diffs.json` is synthesized, not recorded, so that one fixture holds
every kind of file the adapter treats differently. It has two pages of the `/merge_requests/:iid/diffs` shape from
GitLab's API documentation: a normal file, a collapsed one, a binary file too
large to show, and a pure rename.
