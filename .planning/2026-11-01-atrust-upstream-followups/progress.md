# Progress Log

## Session: 2026-11-01

Plan: `.planning/2026-11-01-atrust-upstream-followups/`. Pinned with
`PLAN_ID=2026-11-01-atrust-upstream-followups`.

### Current Status

- **Phase:** 1 - Follow the next service on a non-zero authCheck code
- **Status:** pending (plan approved for review; no code changed)
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
- Confirmed the local gaps that this plan addresses: `authCheck` discards a
  non-zero answer's next service; `reportEnv` sends neither the device MAC nor
  the request signature; `bindAuthDevice` uses the old payload; keep-alive
  lacks name-only domain targets and a route witness; `connectConn` only falls
  back to the major node group.
- Corrected two earlier assumptions while checking the tree: the node probe
  already completes a TLS handshake in `internal/ping/tcp.go` (#124), so the
  mihomo false-positive is covered; and #166 merged on 2026-10-31, after this
  tree's source commit, so the trusted-device fix is an upstream change to
  port rather than an unmerged proposal.

No repository code, tests, documentation or configuration were changed in this
session. Nothing was committed or pushed.

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
