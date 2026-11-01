# Progress Log

## Session: 2026-11-01

Plan: `.planning/2026-11-01-atrust-upstream-followups/`. Pinned with
`PLAN_ID=2026-11-01-atrust-upstream-followups`.

### Current Status

- **Phase:** all five phases complete
- **Status:** implemented, tested and committed on `main`
- **Started:** 2026-11-01
- **Commits:** `f90c873`, `4646e20`, `c22f363`, `0ce8e13`

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

### What was implemented

- `authCheck` parses the step first and follows one the loop can run; the
  terminal steps became one table (`authStepHandlers`) the loop dispatches
  through, and a non-zero code without a usable step is a `PortalCodeError`.
- `reportEnv` sends `mac_addresses` and `X-Request-Sig` over
  `path?query + body`, keyed by the anti-MITM answer's derived key. One locally
  administered MAC is generated, written back before the login can fail, and
  reused.
- The trusted-device bind is the `{unbindList, reason}` exchange with
  `bindStatus`/`curBindNum`/`bindNumLimit`/`trustedDeviceList`; the prompt is
  `authchallenge.CodeDevice` carrying the numbered list. The yes/no
  confirmation path is gone from the protocol layer, `core/session`, the web
  prompter, the window and the CLI.
- The keep-alive derives targets from name-only grants, asks the dialer where a
  probe would go (`RouteFor`) and reports `routed=tunnel|direct`, and treats any
  HTTP answer as live while keeping the status.
- One node-group fallback (`nodeAddress`) for the IP request, the TCP tunnel and
  the L3 tunnel; the IP request budget is 20 s.
- `ResetLoginData`/`RemoveClientData` with a two-step settings row and
  `gotrust-cli --reset-login-data`.

### Test Results

| Test | Expected | Actual | Status |
|------|----------|--------|--------|
| `go build ./...` | clean | clean | pass |
| `go test ./...` | clean | clean | pass |
| `go vet ./...` | clean | clean | pass |
| `gofmt -l .` | empty | empty | pass |
| `reuse lint` | compliant | compliant | pass |
| `gotrust-cli` against a stub gateway | login reaches the bind step, MAC and signature sent | observed: `authCheck answered code 75500503 ... following it`, `bindAuthDevice body = {"reason":"","unbindList":"[]"}`, one MAC reused on the next run | pass |
| `gotrust-cli --reset-login-data` | removes the record | `stored login data removed`, file gone | pass |

### Live-gateway gap

The MAC, the signature formula and the bind answer shapes come from the public
description, not from a live strict gateway. They are pinned by `httptest`
fixtures that encode the recorded answers; a run against a real gateway with
the device limit reached is the one check this session could not make.

### Errors

| Error | Resolution |
|-------|------------|
| GitHub code search API requires authentication | Used the repository issue/comment endpoints instead; complete data was obtained. |

### Next

Nothing in this plan. A future session with a strict gateway should confirm the
`X-Request-Sig` acceptance and the unbind answer on hardware.
