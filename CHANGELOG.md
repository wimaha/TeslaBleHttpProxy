# Changelog

All notable changes are listed here. `Unreleased` is what `main` and the Docker image `:dev` contain; each version heading matches a release tag and the Docker image `:<version>` (the newest one is also `:latest`).

## Unreleased

### Breaking

- `/gen_keys` and `/remove_keys` accept `POST` only. `GET` now returns `405`. The dashboard already uses `POST`; scripts or bookmarks that call these endpoints with `GET` must switch. ([#170](https://github.com/wimaha/TeslaBleHttpProxy/pull/170))

### Security

- State-changing requests a browser sends from another website are rejected with `403`, so a page you visit can no longer change your keys or send commands to the car. evcc, curl and Home Assistant send neither `Origin` nor `Sec-Fetch-Site` and are not affected. ([#170](https://github.com/wimaha/TeslaBleHttpProxy/pull/170))
- Dashboard and log viewer escape all output (`html/template`). ([#170](https://github.com/wimaha/TeslaBleHttpProxy/pull/170))

### Added

- `trustedOrigins` environment variable for reverse proxies over plain HTTP that rewrite the `Host` header, and for web pages on other origins that call the API. See [environment variables](docs/environment_variables.md#trustedorigins). ([#170](https://github.com/wimaha/TeslaBleHttpProxy/pull/170))
- Docker image `:dev`, built from `main` on every push. ([#171](https://github.com/wimaha/TeslaBleHttpProxy/pull/171))
- README examples for `set_charge_limit` and `set_sentry_mode`. ([#169](https://github.com/wimaha/TeslaBleHttpProxy/pull/169))

### Fixed

- A client that disconnects while a response is written no longer stops the proxy. ([#170](https://github.com/wimaha/TeslaBleHttpProxy/pull/170))

### Changed

- Building from source requires Go 1.26 or newer. ([#172](https://github.com/wimaha/TeslaBleHttpProxy/pull/172))
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
