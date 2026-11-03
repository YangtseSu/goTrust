# Findings: platform identity

## Wire behaviour

- The platform query uses `Linux` for every Linux distribution, `Windows` for
  Windows and `macOS` for macOS.
- The login User-Agent states the host platform and architecture; the account's
  `os_family` does not change it.
- A live Linux login with the truthful UA returned code 0 for both environment
  reports and the first tunnel keep-alive answered HTTP 200 (2026-11-03). The
  truthful UA is accepted on the test gateway.

## Main audit

- `core/config/config.go` hardcodes `IdentityOSLinux` in `NewIdentity`,
  `ensureProfileIdentity` and `Profile.normalise`; `macos` is absent from the
  constants and the validator. The config also accepts `uos` and ships the UOS
  UA preset; both are removed, and a stored `uos` is rewritten to `linux`.
- `core/atrust/client/atrust/auth/auth.go` hardcodes `platform=Linux` in
  `sharedParams` and ships `UserAgent` as a Linux fallback.
- `core/atrust/client/atrust/auth/request.go` hardcodes `platform=Linux` in
  `endpointStrategy` and `reportParams`; its `reportEnv` uses the configured UA.
- `core/atrust/client/atrust/tcptunnel.go` hardcodes
  `Platform: "Linux"` in the TCP tunnel process block.
- `core/atrust/client/atrust/l3tunnelconn.go` already derives the process
  platform from `runtime.GOOS` and maps Darwin to `macOS`.
- `core/session/session.go` passes `profile.Ident.UserAgentString()` into the
  client, so the app's production path overrides the auth fallback.

## Implementation notes

- The plan expected three platform query literals. The tree had one,
  `auth.sharedParams`, which every request builder already shared; it now reads
  the session platform, so all endpoints follow the identity through one
  builder instead of three edits.
- The User-Agent is built from the host platform and architecture. `os_family`
  selects the platform query and the tunnel process block.
- The protocol layer no longer ships a UA. `auth.Session` refuses its first
  request with `ErrMissingUserAgent` until the caller names one, and the client
  discovery path takes the UA and platform from the same account identity as
  the login.

## Boundaries

- No gateway address, account, host name or client data belongs in this plan or
  its tests; use the documentation ranges for fixtures.
- The endpoint environment report is a separate item; main does not send it.
