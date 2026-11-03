# Task Plan: one platform identity from config to wire

## Goal

The `platform=` query and the tunnel process block are derived from the
account's `identity.os_family`; the login User-Agent states the host platform
and architecture. A new profile defaults to the host platform (`linux`,
`windows` or `macos`); every Linux distribution is `linux`. The item exists
for Windows and macOS compatibility: the code currently hardcodes Linux. The
`uos` value is removed; a stored `uos` migrates to `linux`. On a Linux host
with the default, every value stays `Linux`, so the change adds no new wire
behaviour on the supported platform.

## Scope

- `core/config`: detect the host family and add the macOS value.
- `core/session`: derive a platform name from the account identity and pass it
  into the protocol layer.
- `core/atrust/client/atrust`: carry the platform in the session, use it in the
  query builders, and use the same identity for the tunnel process block.
- Tests, documentation and cross-builds for the CLI.

## Non-goals

- No new gateway capability, no change to the login flow, and no change to the
  Linux wire values.
- No claim that a synthetic Windows/macOS preset passes a gateway; the preset
  is a wire identity, not a supported runtime.
- No endpoint environment report work: main sends only the report its login
  path builds.
- No new Linux distribution presets; this item only separates Windows and
  macOS from Linux.

## State before this item

| Value | Current source | Problem |
|---|---|---|
| `identity.os_family` default | `NewIdentity`, `ensureProfileIdentity`, `Profile.normalise` hardcode `linux` | A Windows/macOS build reports Linux; macOS has no value |
| `os_family = "uos"` | config constant and UA preset | Claims a Linux/UOS identity; remove it and migrate a stored value to `linux` |
| `platform=` query | `auth.sharedParams`, `endpointStrategy`, `reportParams` hardcode `Linux` | The query disagrees with any non-Linux UA |
| TCP tunnel process block | `tcptunnel.go` hardcodes `Platform: "Linux"` | Every TCP authorization claims Linux; the L3 tunnel already derives `runtime.GOOS` |
| Auth package UA default | `auth.UserAgent` is a Linux browser UA | A caller that does not set a UA reports Linux |

The `reportEnv` on main uses the configured `UserAgentString()`, which is
correct once the family is honest. There is no hardcoded browser UA on the
production path; the `auth.UserAgent` constant is the fallback.

## Decisions

1. **Family vocabulary.** `linux`, `windows` and `macos`. The `uos` value is
   removed. A stored `uos` is read as a legacy value and rewritten to `linux`
   before validation, so an old file still loads.
2. **Host default.** `runtime.GOOS` selects the default: `windows` on Windows,
   `macos` on Darwin, `linux` otherwise. An existing hand-edited value wins,
   except the legacy `uos` value, which becomes `linux`.
3. **One platform per session.** The platform string is a field on the auth
   session, set from the account identity before the first request. The three
   query builders read that field; no platform literal remains in the request
   code.
4. **Tunnel process block follows the identity.** The TCP and L3 tunnels use the
   account's platform mapping, not `runtime.GOOS` directly. On the default Linux
   configuration the result is `Linux`; an explicit preset makes the query and
   the process block agree, while the UA stays host-truthful.
5. **No platform default in the protocol layer.** The auth package stops
   shipping a platform-specific UA. A session without a UA fails with a named
   error before the first request; `core/session` always sets one.

## Platform map

| `os_family` | Login UA | `platform=` | Tunnel process | `endpoint.os.family` |
|---|---|---|---|---|
| `linux` | Host truthful wording | `Linux` | `Linux` | `Linux` |
| `windows` | Host truthful wording | `Windows` | `Windows` | `Windows` |
| `macos` | Host truthful wording | `macOS` | `macOS` | `macOS` |

The three platform strings are `Linux`, `Windows` and `macOS`; the endpoint
family uses the same names. Main does not send that field, but the mapping
stays in one place for that work. The User-Agent is the same for every family
and comes from the host platform and architecture.

## Phases

### Phase 1 — config vocabulary and host default

- [x] Add `IdentityOSMacOS = "macos"`; accept it in `validate`.
- [x] Add `hostOSFamily()` and use it in `NewIdentity`, `ensureProfileIdentity`
      and `Profile.normalise`; a Linux host maps to `linux`.
- [x] Add the macOS family.
- [x] Remove `IdentityOSUOS` and its preset; rewrite a stored `uos` to
      `linux` before validation.
- [x] Tests: host family, macOS family, legacy `uos` migration, validation;
      update the documentation's `os_family` default and value list.

### Phase 2 — per-session platform

- [x] Add `Platform` to `atrust.SessionOptions` and `SetupOptions`, and a field
      on `auth.Session`; set it before the first request.
- [x] `core/session` derives it from `config.IdentityConfig.PlatformName()`.
- [x] Replace the shared `platform` literal with the session value; the builder
      that every request already used now reads the session, so login,
      discovery and every later request agree.
- [x] Tests: the query carries each session platform, and the Linux default
      still sends `Linux`.

### Phase 3 — tunnel process platform

- [x] Add one identity-to-process-platform helper (`Linux`/`Windows`/`macOS`).
- [x] Use it in the TCP tunnel instead of `Platform: "Linux"`.
- [x] Use it in the L3 tunnel instead of the `runtime.GOOS` expression.
- [x] Test the helper; the Linux value is unchanged.

### Phase 4 — auth UA default

- [x] Remove the platform-specific `auth.UserAgent` fallback; a session without
      a UA fails with a named error.
- [x] `core/session` always sets the UA, discovery included; update the auth
      package tests.
- [x] Record the change in `core/atrust/PROVENANCE.md`.

### Phase 5 — gates

- [x] `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .`,
      `reuse lint`.
- [x] `GOOS=windows go build ./cmd/gotrust-cli` and
      `GOOS=darwin go build ./cmd/gotrust-cli`.
- [x] Live Linux login and tunnel keep-alive unchanged (manual).
- [x] Grep gate: no `platform=Linux` or `Platform: "Linux"` literal outside the
      mapping helper and its tests.

## Verification (2026-11-03)

- The five gates above pass; both cross-builds produce a binary.
- A live login with the default Linux identity logged in with code 0, the two
  environment reports and the resource fetch succeeded, and the first
  keep-alive answered HTTP 200 through the tunnel.
- The only `"platform": {"Linux"}` values left are wire expectations in tests;
  production code reads the session.

## Risks and open questions

- The `macOS` and `Windows` query values need a live run on Windows and macOS
  before a release claims exact parity.
- Removing `uos` changes an existing config value; the migration to `linux`
  keeps the file loadable and the Linux wire values unchanged.
- The tunnel process block for an explicit preset follows the preset rather
  than the host. That is the consistent wire identity, but it makes the process
  block less literal; the default on every host is unchanged.
- The User-Agent always states the host, while `platform=` and the process
  block follow the account. An explicit preset therefore reports a host UA
  with a preset platform; that is intentional.
- The auth package refactor touches many call sites. Keep the existing helper
  signatures where possible and cover the query builders with tests.
- Removing the UA default changes a package-level behaviour; no goTrust caller
  depends on it, but direct users of the auth package do.

## Acceptance

- On a Linux host with the default, the UA, `platform=`, and process block are
  all Linux; a live login and keep-alive succeed.
- With `os_family = "windows"` or `"macos"`, the query and the process block
  agree; the UA still states the host.
- A Windows/macOS build defaults to the matching family instead of Linux.
- No platform literal remains outside the map; the full gate and the cross-build
  checks pass.
