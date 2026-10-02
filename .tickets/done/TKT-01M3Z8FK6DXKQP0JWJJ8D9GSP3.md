---
schema: 3
id: TKT-01M3Z8FK6DXKQP0JWJJ8D9GSP3
title: Confirm before submitting or approving, unless the person waives it
type: task
status: done
status_reason: null
priority: high
due_on: null
labels:
  - ui
  - gitlab
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T21:31:47Z
updated_at: 2026-10-02T21:35:36Z
created_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

The owner clicked "Submit and approve" on next-gen-compute!144 by accident on 2026-10-02. It went straight to GitLab, where the approval failed with a 401 (they are the MR's author), and the error read "401 Unauthorized: 401 Unauthorized".

Ask before an action that publishes to the forge as the person: Submit, Submit and approve, Approve, and Revoke approval. The dialog says what will happen, as whom, and on which merge request, and Cancel is the default. A "Don't ask again" box waives the confirmation for that action. Waivers persist in vuoro's state directory, and the home page lists them, with a way to ask again.

Also: an approval refused with 401 says why that usually happens, and GitLab's message is not repeated when it only restates the status.

## Acceptance criteria

- [x] Submit, Submit and approve, Approve, and Revoke approval ask first, and Cancel does nothing, which a test proves
- [x] A waived action runs without asking, the waiver survives a restart, and the home page can take it back
- [x] A 401 on approve says what it usually means

## Implementation plan

A SubmitBar component owns the four actions and asks through an in-page dialog, Cancel focused, Escape and a click outside cancelling. Waivers are per action (submit, approve), kept server-side in prefs.json in the state directory, because the page's origin changes every launch and the config file is the person's to write. /api/setup reports them; POST /api/prefs/confirm sets them; the home page takes them back. A 401 on approve is explained in the GitLab client.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T21:35:36Z

Checked in the dev copy on !144: Submit and approve opens the dialog naming !144 and @drewshort, warns that the person opened it, focuses Cancel, and Cancel posts nothing. Not confirmed in the browser, because that copy holds the real token. web/src/confirm.test.tsx covers each action asking, Cancel and Escape posting nothing, a waived action running at once, and the tick waiving. app/prefs_test.go covers the waiver surviving a restart and being taken back. The 401 on !144 was most likely GitLab refusing the author's own approval, not the token.

## Summary

Submit, Submit and approve, Approve, and Revoke approval now ask first, unless the person waives submitting or approving; waivers persist in prefs.json and the home page takes them back. A refused approval explains GitLab's 401.
