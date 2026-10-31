# Findings: aTrust upstream-log followups

This file records what the two upstream issue trackers actually printed. It is
the evidence base for `task_plan.md`; every claim below has a link. Counts:
**226 issues/PRs**, **596 issue comments**, **92 review comments** and **5
uploaded logs** across `chenx-dust/EZ4Connect` and
`Mythologyli/zju-connect`. Log-bearing items were read in full; screenshots
were not OCR'd. One attached log contains a real account, password and phone
number; no real address or credential is reproduced here.

## 1. Session lifetime and keep-alive

Measured on an aTrust 2.5.16 gateway (`auth/psw`) in
[#154](https://github.com/Mythologyli/zju-connect/issues/154) and
[#155](https://github.com/Mythologyli/zju-connect/pull/155):

- An idle session expires about **10 minutes** after the last application
  traffic (three runs: 11.0 / 10.7 / 10.5 min; one longer observation: 17 min).
- Periodic `authConfig` / `onlineInfo` reads **do not** extend it.
- A plain HTTP request to a granted application every 60 s held the session for
  **over one hour** (`KeepAlive: OK status=200 target=...` once a minute).
- An `https://` target is reached with CONNECT in that deployment and is **not**
  application traffic; it does not renew the session.
- The renewal default was 30 min, which cannot hold a 10 min session. The
  replacement is 60 s (minimum 30, `0` disables).

[#162](https://github.com/Mythologyli/zju-connect/pull/162) reduced the
keep-alive to named `http://` targets plus derived targets, and restricted the
derived set to grants covering **port 80**. A single-port 22/443 grant is not
an HTTP service; probing it with plain HTTP failed and consumed a candidate
slot. The plan therefore keeps 80 as HTTP, but probes 443 with a TLS handshake
and 22 by reading the SSH identification.

EZ4Connect [#59](https://github.com/chenx-dust/EZ4Connect/issues/59) reaches the
same conclusions and notes the UI always passed `-disable-keep-alive`; the log
in EZ4Connect [#56](https://github.com/chenx-dust/EZ4Connect/issues/56) shows
the parameter. A DNS keep-alive can print `KeepAlive using UDP: OK` while the
session dies; the log in
[zju-connect #127](https://github.com/Mythologyli/zju-connect/issues/127)
(attachment `log-tun.log`) shows nine minutes of DNS OK before the resource
issue, and the original #60 log shows the same pattern over hours.

## 2. `authCheck` and the trusted-device login path

[zju-connect #96](https://github.com/Mythologyli/zju-connect/issues/96) and
[#120](https://github.com/Mythologyli/zju-connect/issues/120) record a strict
gateway answering:

```text
Received auth check: {"code":75500503,"message":"The client is not installed,
or login in web-only mode is performed. No trusted endpoint can be added...",
"data":{"currentService":"auth/authCheck","type":"trustDevice",
"nextService":"auth/bindAuthDevice",
"nextServiceList":[{"authId":"-1","authType":"auth/bindAuthDevice"}]}}
```

The old client treated the non-zero code as fatal and then tried
`auth/sms`, which answered `code:10000006` (page not found). The portal had
already named the next step: `auth/bindAuthDevice`.

Local code (`core/atrust/client/atrust/auth/request.go`, `authCheck`) does the
same premature error return:

```go
if ac.Code != 0 {
    return authStep{}, fmt.Errorf("authCheck failed with code %d: %s", ac.Code, ac.Message)
}
s.capturePasswordChange(...)
return authStepFromData(ac.Data), nil
```

The anti-MITM data is already parsed in `authConfigContext`; the local helpers
`antiMITMSignatureKey` and `antiMITMHMAC` can produce the request signature.
The `continueAuth` switch already knows `auth/bindAuthDevice`, but the current
implementation posts `{deviceId, authId}`; the recorded flow is different (see
below).

[#166](https://github.com/Mythologyli/zju-connect/pull/166) is the upstream
attempt at the same path. Its changes:

- `ClientAuthData.MACAddresses`, `LoginOptions.MACAddresses`, and a
  `randMACAddress` that sets the locally-administered/unicast bits.
- `reportEnv` sends `mac_addresses` in `env.endpoint.device` and an
  `X-Request-Sig` header:
  `HMAC(signKey, "/controller/v1/public/reportEnv?"+query+body)`.
- `bindAuthDevice` posts `{unbindList:"[]", reason:""}`, reads
  `bindStatus`/`curBindNum`/`bindNumLimit`/`trustedDeviceList`, asks the user
  for `max(1, curBindNum-bindNumLimit+1)` numbers, posts the selected IDs and
  follows the next service.
- A new `authchallenge.CodeDevice` kind is used for the list prompt.

The PR was **merged on 2026-10-31**, after this tree's source commit
(2026-10-24), so the local tree predates the fix. Its exact field names are the
best recorded description; the merge is not a live result on every gateway.

## 3. Node selection and false reachability

[zju-connect #123](https://github.com/Mythologyli/zju-connect/issues/123): with
mihomo TUN enabled, the proxy answers the TCP connect locally, so every node
looks reachable and the best-node scores are fake. Upstream merged
[#124](https://github.com/Mythologyli/zju-connect/pull/124), which makes the
TCP probe finish with a TLS handshake. **The local tree already carries that
change**: `core/atrust/internal/ping/tcp.go` dials, then calls
`tlsConn.HandshakeContext` before returning, so `node.go`'s `ping.TCP` is not
fooled by a proxy that only answers the connect. No code change is required;
keep a regression test only if a future edit touches the probe.

## 4. Keep-alive target coverage and truthfulness

- `DeriveKeepAliveTargets(ipResources)` only sees `IPResource`. A name-only
  domain grant (no `address.IP`) has no target. The parser adds an
  `IPResource` for each `address.IP` entry, so most domain apps are covered;
  name-only grants are the gap.
- [#155](https://github.com/Mythologyli/zju-connect/pull/155) proposed
  `routed=tunnel|direct` in the probe line; a target that should be tunneled
  but goes direct is a configuration error. The reduced #162 dropped the
  witness, so a named target outside the VPN can report an HTTP answer from a
  direct connection and look like a successful keep-alive.
- The local `httpKeepAliveAttempt` treats `>=400` as failure. A 4xx/5xx answer
  still means the request crossed the tunnel; for session renewal that is
  application traffic. The status code should stay visible but not decide
  liveness.

## 5. Small connection defects

- EZ4Connect [#19](https://github.com/chenx-dust/EZ4Connect/issues/19) logs
  `Major node group not found, using <node> instead`. Local `getIP` falls back
  to any group; `L3Tunnel.connectConn` only falls back to `MajorNodeGroup`.
- [zju-connect #108](https://github.com/Mythologyli/zju-connect/issues/108)
  shows `requestIP` failing after a 20 s dial timeout. Local `getIP` uses a
  10 s context.
- EZ4Connect [#36](https://github.com/chenx-dust/EZ4Connect/issues/36): a
  corrupt/stale `client_data` produced repeated `already on`/`EOF`; removing
  the file fixed it. Local profile deletion removes the file, but there is no
  dedicated "reset login data" action.
- [zju-connect #99](https://github.com/Mythologyli/zju-connect/issues/99): a
  browser QUIC/UDP flood triggered `invalid SID` and a panic in
  `tcpTunnelConn.Read`. The local tree classifies `invalid SID` and recovers;
  no UDP rate limit exists.

## 6. Deployment / another proxy in front

- [zju-connect #54](https://github.com/Mythologyli/zju-connect/issues/54) and
  [#55](https://github.com/Mythologyli/zju-connect/pull/55) add
  `dial_direct_proxy` so direct traffic can leave through an HTTP/SOCKS proxy;
  [#114](https://github.com/Mythologyli/zju-connect/pull/114) adds interface
  binding to avoid a traffic loop. EZ4Connect #56 passes
  `-dial-direct-proxy` in its core command line.
- The local `dial.NewDialer(vpnStack, resolver, ipResources, false, "")` has no
  configured direct proxy and no interface binding.

## 7. Already covered locally (verify, do not re-implement)

- `invalid SID` / `10000004` / `75500002` are classified and the session
  recovery ladder runs (`tcptunnel.go` → `reportIfSessionInvalid` →
  `runSessionRecovery`).
- `aTrust session state` and `requires reauthentication` lines are emitted and
  mapped by the GUI.
- L3 auth timeout has a sentinel and a retry (`errL3TunnelAuthTimeout`).
- The TCP tunnel waits for the full SOCKS5 connect reply
  (`waitForTCPConnectReply`), including refused/unreachable errors.
- Listener bind failures are returned, not panics.
- Node addresses without a port get `:441` in the parser.
- RADIUS, SMS, CAS, `auth/httpsOauth2`, forced password change, `sidTicket`
  restore, terminal trust/untrust APIs and debug redaction are present.
- gVisor `Run` panics are recovered by `runPacketStack`.
- The node probe in `internal/ping/tcp.go` already completes a TLS handshake
  after the connect (#124), so a TCP-only proxy answer is not counted.
- Domain rules are appended per host and port; the upstream 80-over-443 bug
  does not exist locally.

## 8. Not applicable

- EasyConnect-only: `0-bit RSA`, `No CSRF rand code`, TOTP answered with HTML,
  `/por/update_session.csp`, `SHUTDOWN cmd 0x08`, port-forwarding
  `panic: ... is not resource`.
- TUN-only: wintun zero-length packet panic, route loops, fake-IP DNS rules.
- EZ4Connect's core/UI literal matching: goTrust is one process and does not
  parse its own output for prompts.

## 9. Security note

`zju-connect #127`'s attachment `log-tun.log` contains a plaintext gateway
account, password and phone number. It is public. Do not copy it into a test,
commit or report; consider asking the uploader to redact it. The same applies
to any future attachment: sanitize before quoting.

## 10. Resources

- #154: <https://github.com/Mythologyli/zju-connect/issues/154>
- #155: <https://github.com/Mythologyli/zju-connect/pull/155>
- #162: <https://github.com/Mythologyli/zju-connect/pull/162>
- #96: <https://github.com/Mythologyli/zju-connect/issues/96>
- #120: <https://github.com/Mythologyli/zju-connect/issues/120>
- #166: <https://github.com/Mythologyli/zju-connect/pull/166>
- #123: <https://github.com/Mythologyli/zju-connect/issues/123>
- #124: <https://github.com/Mythologyli/zju-connect/pull/124>
- #19 (EZ4): <https://github.com/chenx-dust/EZ4Connect/issues/19>
- #36 (EZ4): <https://github.com/chenx-dust/EZ4Connect/issues/36>
- #56 (EZ4): <https://github.com/chenx-dust/EZ4Connect/issues/56>
- #59 (EZ4): <https://github.com/chenx-dust/EZ4Connect/issues/59>
- #108: <https://github.com/Mythologyli/zju-connect/issues/108>
- #99: <https://github.com/Mythologyli/zju-connect/issues/99>
- #54: <https://github.com/Mythologyli/zju-connect/issues/54>
- #55: <https://github.com/Mythologyli/zju-connect/pull/55>
- #114: <https://github.com/Mythologyli/zju-connect/pull/114>
