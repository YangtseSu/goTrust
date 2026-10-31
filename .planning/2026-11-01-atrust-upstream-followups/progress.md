# Progress Log

## Session: 2026-11-01

Plan: `.planning/2026-11-01-atrust-upstream-followups/`. Pinned with
`PLAN_ID=2026-11-01-atrust-upstream-followups`.

### Current Status

- **Phase:** 1 - Follow the next service on a non-zero authCheck code
- **Status:** pending (public-source plan; no code changed)
- **Started:** 2026-11-01

### What was done

- Read the planning skill and initialized this plan under `.planning/`.
- Enumerated every issue and PR in `chenx-dust/EZ4Connect` and
  `Mythologyli/zju-connect` through the GitHub API: 226 items, 596 issue
  comments, 92 review comments, 5 uploaded attachments.
- Extracted log-bearing items, read the key ones in full and wrote the
  evidence to `findings.md`.
- Confirmed the local code already covers the session-invalid recovery, the
  L3 auth timeout, the TCP-tunnel connect status, listener bind errors, the
  missing node port default, RADIUS/SMS/CAS/OAuth2, forced password change,
  `sidTicket` restore and debug redaction.
- Corrected two earlier assumptions while checking the tree: the node probe
  already completes a TLS handshake in `internal/ping/tcp.go` (#124), so the
  mihomo false-positive is covered; and #166 merged on 2026-10-31, after this
  tree's source commit, so the trusted-device fix is a public change to port.

### Scope decision

The plan uses only the public issue/PR evidence. It keeps:

- `authCheck` non-zero code with a next service;
- the public #166 `reportEnv` MAC/request signature and trusted-device bind
  flow;
- keep-alive truthfulness;
- the small connection defects.

No repository code, tests, documentation or configuration were changed in this
session. The plan files are in the working tree and were not committed.

### Test Results

| Test | Expected | Actual | Status |
|------|----------|--------|--------|
| (none this session) | - | - | - |

### Errors

| Error | Resolution |
|-------|------------|
| GitHub code search API requires authentication | Used the repository issue/comment endpoints instead; complete data was obtained. |

### Next

Start Phase 1 with the failing `authCheck` fixture, then make the smallest
change that follows the recorded `nextService`. Run
`go test ./core/atrust/client/atrust/auth` before moving on.
