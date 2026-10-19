# Changelog

All notable changes are listed here. `Unreleased` is what `main` and the Docker image `:dev` contain; each version heading matches a release tag and the Docker image `:<version>` (the newest one is also `:latest`).

## Unreleased

## 2.5.0 - 2026-10-19

### Upgrade notes

Normal setups with evcc or Home Assistant are not affected and need no changes. The points below only matter in the cases they name.

- If you open the dashboard through a reverse proxy over plain HTTP that rewrites the `Host` header, its buttons now answer `403`. Set [`trustedOrigins`](docs/environment_variables.md#trustedorigins) to the address you open in the browser. Over HTTPS nothing needs to be set. ([#170](https://github.com/wimaha/TeslaBleHttpProxy/pull/170))
- `/gen_keys` and `/remove_keys` accept `POST` only. `GET` now returns `405`. The dashboard already uses `POST`; only scripts or bookmarks that called these undocumented endpoints with `GET` must switch. ([#170](https://github.com/wimaha/TeslaBleHttpProxy/pull/170))
- Requests that fail are now always reported as failures (`result: false`, HTTP 503). Before, some of them, for example `set_charging_amps` without a body, returned `result: true` without doing anything. Commands sent without `wait=true` that cannot be queued (queue full, key change) now return 503 instead of "will be processed shortly". `body_controller_state` waits in the command queue and answers with an error after 15 seconds if the proxy is busy, for example retrying while the car is out of range. ([#174](https://github.com/wimaha/TeslaBleHttpProxy/pull/174))

### Security

- State-changing requests a browser sends from another website are rejected with `403`, so a page you visit can no longer change your keys or send commands to the car. evcc, curl and Home Assistant send neither `Origin` nor `Sec-Fetch-Site` and are not affected. ([#170](https://github.com/wimaha/TeslaBleHttpProxy/pull/170))
- Dashboard and log viewer escape all output (`html/template`). ([#170](https://github.com/wimaha/TeslaBleHttpProxy/pull/170))

### Added

- Vehicle status on the dashboard: connection and sleep state, battery and charging, climate, lock state, last contact and last error per vehicle, each with the time its data is from. It updates itself every 10 seconds from what the proxy already knows and never contacts the vehicle on its own; **Refresh** fetches current data without waking it, **Wake & refresh** wakes it first. New endpoints `GET /api/proxy/1/status` and `POST /api/proxy/1/status/refresh`. ([#168](https://github.com/wimaha/TeslaBleHttpProxy/issues/168), [#176](https://github.com/wimaha/TeslaBleHttpProxy/pull/176))
- `trustedOrigins` environment variable for reverse proxies over plain HTTP that rewrite the `Host` header, and for web pages on other origins that call the API. See [environment variables](docs/environment_variables.md#trustedorigins). ([#170](https://github.com/wimaha/TeslaBleHttpProxy/pull/170))
- Docker image `:dev`, built from `main` on every push. ([#171](https://github.com/wimaha/TeslaBleHttpProxy/pull/171))
- Docker image `:pr-<number>` for every pull request from a branch of this repository, to test a change on a car before it is merged. See [Test images for pull requests](README.md#test-images-for-pull-requests). ([#175](https://github.com/wimaha/TeslaBleHttpProxy/pull/175))
- `releaseAdapterWhenIdle` environment variable: hands the Bluetooth adapter back to the system between connections, so other Bluetooth services on the same host can use it. Off by default. Fixes [#158](https://github.com/wimaha/TeslaBleHttpProxy/issues/158), based on [#161](https://github.com/wimaha/TeslaBleHttpProxy/pull/161) by @AloisKlingler. ([#174](https://github.com/wimaha/TeslaBleHttpProxy/pull/174))
- `connectionTimeout` environment variable: how long a connection stays open after a command (default 29 seconds as before). Based on [#161](https://github.com/wimaha/TeslaBleHttpProxy/pull/161) by @AloisKlingler. ([#174](https://github.com/wimaha/TeslaBleHttpProxy/pull/174))
- README examples for `set_charge_limit` and `set_sentry_mode`. ([#169](https://github.com/wimaha/TeslaBleHttpProxy/pull/169))

### Fixed

- `set_charging_amps`, `set_charge_limit` and `set_sentry_mode` with a missing or invalid body answer with an error before the car is contacted. Before, they could report success without doing anything. Other errors that are not retried are reported too instead of coming back as success. ([#174](https://github.com/wimaha/TeslaBleHttpProxy/pull/174))
- `body_controller_state` goes through the command queue: it no longer opens a second Bluetooth connection next to a running command, and it is answered right away on an open connection. ([#174](https://github.com/wimaha/TeslaBleHttpProxy/pull/174))
- Changing or generating keys in the dashboard stops the previous command loop instead of leaving it running next to the new one. Requests during the change get a clear error instead of hanging. ([#174](https://github.com/wimaha/TeslaBleHttpProxy/pull/174))
- A client that disconnects while a response is written no longer stops the proxy. ([#170](https://github.com/wimaha/TeslaBleHttpProxy/pull/170))

### Changed

- `body_controller_state` waits in the command queue like every other request. If the proxy is busy with another command or retrying a connection, it answers with an error after at most 15 seconds instead of opening a second connection. A caller that gave up is not sent to the vehicle later. ([#174](https://github.com/wimaha/TeslaBleHttpProxy/pull/174))
- Errors are reported more strictly: requests that failed without being retried (for example a missing body) now return `result: false` (HTTP 503), where some used to return `result: true`. ([#174](https://github.com/wimaha/TeslaBleHttpProxy/pull/174))
- When the command queue is full (50 waiting requests, the vehicle has been unreachable for a while), new requests are answered with an error right away instead of waiting. ([#174](https://github.com/wimaha/TeslaBleHttpProxy/pull/174))
- Sending a key to the vehicle from the dashboard closes an idle connection of the command loop first and waits at most 30 seconds for a busy one. ([#174](https://github.com/wimaha/TeslaBleHttpProxy/pull/174))
- Updated BLE library ([wimaha/ble_BleConnectFix#1](https://github.com/wimaha/ble_BleConnectFix/pull/1)): when the proxy closes the adapter, it is brought back up, so BlueZ sees it again. Needed for `releaseAdapterWhenIdle`. ([#174](https://github.com/wimaha/TeslaBleHttpProxy/pull/174))
- Building from source requires Go 1.27 or newer; the Docker image is built with Go 1.27.2. ([#172](https://github.com/wimaha/TeslaBleHttpProxy/pull/172), [#178](https://github.com/wimaha/TeslaBleHttpProxy/pull/178))
- Tests run against a simulated vehicle, so commands, connection handling and the HTTP API are covered without a car. ([#169](https://github.com/wimaha/TeslaBleHttpProxy/pull/169))

## 2.4.0 - 2026-10-17

Changes up to and including 2.4.0 are described in the [GitHub releases](https://github.com/wimaha/TeslaBleHttpProxy/releases).

## Releasing

Before you push a release tag:

1. Rename `## Unreleased` to `## <version> - <date>` and add a new, empty `## Unreleased` above it.
2. In README and `docs/`, replace every marker line `*(unreleased)*` with `*(since <version>)*`.
3. Raise the version in the note at the top of the README to `<version>` (text and link).
4. Commit, then tag `<version>` (without `v`, like `2.4.0`).

The release workflow checks these steps and stops before publishing `:latest` if one is missing.
