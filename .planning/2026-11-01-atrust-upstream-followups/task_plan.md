# Task Plan: aTrust upstream-log followups

## Goal

Close the aTrust login and session gaps that the upstream issue logs expose,
starting with the trusted-device login path: follow a non-zero `authCheck`
answer's next service, make `reportEnv` carry the device MAC and request
signature, and complete the trusted-device bind flow. Then make keep-alive
results truthful and fix the small connection defects recorded in
`findings.md`. The node TLS probe was already present in
`internal/ping/tcp.go` (#124), so it needs verification, not new code. No
EasyConnect, TUN or port-forwarding work.

## Next Step

Add the failing `authCheck` fixture: a `code:75500503` answer that carries
`nextService:auth/bindAuthDevice`, and assert the login loop reaches the bind
step instead of returning the code as an error.

## Current Phase

Phase 1 (pending).

## Decisions already made

- **The first cut is Phases 1-3.** They are one login path: a strict gateway
  answers `authCheck` with `code:75500503` and a bind step, and rejects a
  `reportEnv` that lacks the MAC and request signature. Splitting them leaves
  the path half-working.
- **Phases 4-5 are queued, Phase 6 is out of scope for now.** They are
  independent and can be cut later; nothing in Phases 1-3 depends on them.
- **No silent re-login.** `relogin-required` stays terminal; `auto-relogin`
  stays off by default. A full login rotates the device token and drops the
  account's other clients (upstream #159, EZ4 #62).
- **The MAC is persistent, not per login.** It is generated once (locally
  administered, unicast) and stored in the profile's client data, so the portal
  sees one device across restarts. A missing value is filled in, never
  regenerated.
- **The node TLS probe is already implemented.** `internal/ping/tcp.go` does a
  TLS handshake after the TCP connect (#124), so `node.go`'s `ping.TCP` is not
  fooled by a proxy that only answers the connect. It stays covered; no phase
  changes it.
- **A keep-alive probe that reached the application is a live probe.** For an
  HTTP target any response (including 4xx/5xx) proves traffic crossed the
  tunnel; the status code is a detail, not the success criterion. A direct
  probe is never a successful keep-alive.
- **Unit fixtures first, a live gateway second.** The trusted-device and
  signature changes are protocol-sensitive; each phase lands with an
  `httptest` fixture that encodes the recorded answer. A live check on a
  gateway that requires a trusted device is an operator step, not a CI step.
- **No upstream issue/PR replies are part of this plan.** The evidence links
  stay in `findings.md`; replying to the upstream projects is a separate
  decision.

## Phases

Status values are `pending`, `in_progress`, `complete` only.

### Phase 1: Follow the next service on a non-zero authCheck code

`authCheck` currently returns an error as soon as `code != 0`, before reading
`Data.NextService`. The recorded answer from a strict gateway is
`code:75500503` with `nextService:auth/bindAuthDevice`; the client must follow
the step, not treat the code as fatal.

- [ ] Add the failing test: a stub `authCheck` answer with
      `code:75500503`, `nextService:auth/bindAuthDevice` and
      `nextServiceList:[{authId:"-1",authType:"auth/bindAuthDevice"}]`;
      assert `continueAuth` reaches `bindAuthDevice`.
- [ ] In `authCheck`, parse the step first. Follow it when the code is non-zero
      **and** the next service is a step `continueAuth` knows. Return the
      portal's code and message only when there is no usable next step.
- [ ] Keep `capturePasswordChange` working for an answer that carries both a
      non-zero code and a password-change step.
- [ ] Add the negative test: a non-zero code with no next service still fails
      with the portal message.
- [ ] Add the regression test: a code-0 answer with a next service is
      unchanged.
- **Status:** pending.

Acceptance: the new fixture passes, the existing auth tests pass unchanged, and
`go test ./core/atrust/client/atrust/auth` is clean.

Risk: a real error may carry a stale `nextService`. Follow only services the
loop implements; an unknown service stays an error.

### Phase 2: reportEnv carries the device MAC and request signature

The official client reports a device MAC and signs `reportEnv`; the recorded
strict-gateway answer rejects a request that omits them. The sign key is
already derivable from the anti-MITM answer.

- [ ] Add `MACAddresses` to `ClientAuthData`, `LoginOptions`, `Session`,
      `ClientOptions` and `LoginResult`; thread it through `core/session` and
      the client-data round trip.
- [ ] Generate a locally administered unicast MAC when the stored client data
      has none, persist it, and reuse it. Do not generate one per login.
- [ ] Add `mac_addresses` to the `reportEnv` device object.
- [ ] Derive the sign key from the anti-MITM answer with the existing
      `antiMITMSignatureKey` helper and add
      `X-Request-Sig` over `path?query + body` to `reportEnv`.
- [ ] Tests: the request body contains the stored MAC; the header is the
      recomputed HMAC; a second login reuses the MAC; old client data without
      the field is upgraded once.
- **Status:** pending.

Acceptance: the stub `reportEnv` verifies both fields; the client-data JSON
round-trip keeps the MAC; a missing MAC is written back exactly once.

Risk: the signature formula is protocol-sensitive. Reuse the local anti-MITM
helpers and pin the recorded path, query and body in the fixture.

### Phase 3: Complete the trusted-device bind flow

The bind step is not the current `{deviceId, authId}` request. The recorded
flow posts `{unbindList, reason}`, and when the device limit is reached it
returns `bindStatus`, `curBindNum`, `bindNumLimit` and `trustedDeviceList`;
the user then picks devices to unbind.

- [ ] Add the bind request and response types; post `unbindList:"[]"` and
      `reason:""` first.
- [ ] When `bindStatus == 1`, ask for
      `max(1, curBindNum-bindNumLimit+1)` device numbers from
      `trustedDeviceList`, post the selected IDs, and follow the returned next
      service.
- [ ] Add `authchallenge.CodeDevice` and wire the prompt through
      `core/session` and both front-ends. The prompt lists the devices and
      their bind time.
- [ ] Tests: `bindStatus=0` follows the next service; `bindStatus=1` with two
      devices unbinds one; a wrong count or an invalid number fails; a
      missing list fails with a clear message.
- **Status:** pending.

Acceptance: a stub portal completes the challenge; no unbind request is sent
when the limit is not reached.

Risk: the exact field names and the prompt wording. Pin the recorded answer in
the fixture and keep the message in the existing challenge UI.

### Phase 4: Keep-alive truthfulness

- [ ] Derive targets from name-only domain grants and apps as well as
      `IPResource`; a grant with no address list still gets a target.
- [ ] Add a routing witness: a named target that the dialer sends direct is
      reported as `routed=direct` and is not a successful keep-alive. Derived
      targets must report `routed=tunnel`.
- [ ] Count any HTTP response as a live probe; keep the status code in the
      result and the UI detail. A transport error is still a failure.
- [ ] Tests: a name-only domain grant yields a target; a direct named target
      is a warning; a 403 target is live; a dial error is not.
- **Status:** pending.

Acceptance: the state and the window distinguish `tunnel` from `direct`, and a
4xx answer no longer moves the round to the next candidate.

Risk: the dialer does not currently expose its route decision. Prefer a small
`Matches`/`Route` query over parsing the `-> VPN` log line.

### Phase 5: Small connection defects

- [ ] `L3Tunnel.connectConn` falls back to any reachable node group, as
      `getIP` already does, instead of only `MajorNodeGroup`.
- [ ] `getIP` uses the 18-20 s setup budget (or a configurable value) instead
      of 10 s, so a slow gateway does not fail setup.
- [ ] Add a "reset login data" action (settings and CLI) that removes the
      profile's client data after confirmation; it is the recovery step for a
      stale record that causes `already on`/EOF.
- **Status:** pending.

### Phase 6: Out of scope for this plan

- UDP/QUIC flood limiting. The session now recovers from `invalid SID`; a rate
  limit needs its own design.
- `dial_direct_proxy` and underlay interface binding. Useful behind Clash
  TUN, but a separate feature with its own config and tests.
- Port-bind error hints, EasyConnect-specific work, TUN-only work and the
  UI/core literal matching fixes.

## Out of scope

- Any EasyConnect protocol work: `0-bit RSA`, `No CSRF rand code`, TOTP HTML,
  `/por/update_session.csp`, `SHUTDOWN cmd 0x08`.
- TUN devices, route installation, fake-IP DNS or wintun.
- Port forwarding listeners and their `resource not found` panic.
- Silent credential replay or turning `auto-relogin` on by default.
- Upstream issue/PR replies.
- Real gateway addresses, accounts or client data in tests, commits or
  screenshots.
