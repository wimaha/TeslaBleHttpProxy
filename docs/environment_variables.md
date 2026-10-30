# Environment variables

> **Note:** This page describes the current development state (`main`, Docker `:dev`). Sections marked *(unreleased)* are not in the latest release yet, see the [CHANGELOG](../CHANGELOG.md).

You can optionally set environment variables to override the default behavior.

## logLevel

This is the log level. Options: debug (Default: info)

## scanTimeout

This is the number of seconds to scan for BLE devices. If set to 0, the scan will continue until a device is found or the context is cancelled. (Default: 5) If the vehicle is sometimes not found, consider increasing this value.

## cacheMaxAge

This is the number of seconds for the HTTP Cache-Control header `max-age` value. It is used for body controller state responses to tell HTTP clients (browsers, proxies, etc.) how long they can cache the response. If set to 0, cache headers are disabled (no-cache). Note: This does not affect VehicleData caching, which uses `vehicleDataCacheTime` instead. (Default: 5)

## vehicleDataCacheTime

This is the number of seconds to cache VehicleData endpoint responses in memory. Each endpoint (e.g., `charge_state`, `climate_state`) is cached separately per VIN, allowing efficient serving of frequently requested vehicle data without establishing a BLE connection. If a request is made within the cache time, the cached response is returned immediately. If set to 0, in-memory caching is disabled. (Default: 30)

## serveStaleWhenAsleep

*(unreleased)*

By default a `vehicle_data` request for a sleeping car whose cache has expired (see `vehicleDataCacheTime`) is answered with HTTP 503 "vehicle is sleeping". If `true`, the proxy answers with the last cached values instead, as long as every requested endpoint has been cached at least once and the oldest value is not older than **24 hours** (fixed, not configurable). The car is never woken by this. The answer is marked so that clients that look for it can tell: `"stale": true` and `"cache_age_seconds"` (age of the oldest value) next to `result`, and the `reason` says that the data is old. The values themselves also carry their own `timestamp`. If an endpoint was never cached, or the data is older than 24 hours, the request fails with 503 as before. Two cases are affected: the car is asleep ("vehicle is sleeping"), and its sleep status could not be read while the request does not wake it ("vehicle sleep status unknown", for example while the car is waking up or the Bluetooth connection is unstable). Other errors, for example a failed command, are reported as before. Because of the second case, a real Bluetooth problem can also show cached values for up to 24 hours instead of an error: watch the proxy logs and the dashboard if the values never change. (Default: false)

> **Warning:** a stale answer is still `HTTP 200` with `"result": true`. **evcc and Home Assistant do not look at `stale`**, so they show the old values (charging state, battery level, range) as if they were current. Whatever the car did meanwhile, for example being unplugged, is not reflected until it wakes up. Values of up to 24 hours old are served as current, and do not use this option for automations that must react to the real state of the car.

## wakeWhenNotCached

*(unreleased)*

By default the car is only woken when the request has `wakeup=true`. If `true`, a `vehicle_data` request that includes an endpoint that has never been cached wakes the car, so that the cache gets filled. This happens **at most once per hour for each endpoint** (and VIN), whether or not the read succeeded: an endpoint that keeps failing wakes the car again only after an hour, not on every poll. You can also send a request with `wakeup=true` at any time. Later requests for an endpoint that is cached do not wake the car, even when the entry has expired. Combine with `serveStaleWhenAsleep` to keep serving those values while the car sleeps.

> **Warning:** this option wakes the car without the client asking for it. The cache and the record of earlier wake-ups are kept in memory only and are **lost when the proxy restarts** (container update, host reboot, crash, `restart: always`). After every restart the car is woken again, once per endpoint that a client requests. If the proxy restarts often, or the car sleeps a lot, leave this option off. Waking the car regularly keeps it from sleeping and uses battery.

(Default: false)

## httpListenAddress

This is the address and port to listen for HTTP requests. (Default: :8080)

## trustedOrigins

*(since 2.5.0)*

Comma-separated list of origins (`scheme://host[:port]`) that may send state-changing requests (dashboard buttons, commands) from a browser. Requests from evcc, curl or Home Assistant integrations are not affected. (Default: empty)

You need it in two cases, because the proxy otherwise answers `403 Forbidden`:

- The dashboard is reached through a reverse proxy over plain HTTP that rewrites the `Host` header (nginx does this by default). Set it to the address you type in the browser, e.g. `http://nas.local:8080`.
- A custom web page or browser tool on another origin calls the API with `POST`, e.g. a page served from `http://homeserver:3000`. Set it to that page's origin.

Case, a trailing slash and the default ports `:80`/`:443` do not matter. Several origins: `trustedOrigins=http://nas.local:8080,https://home.example.com`.

## connectionTimeout

*(since 2.5.0)*

The number of seconds a Bluetooth connection to the vehicle stays open after a command, so that further commands (for example from evcc) are sent without connecting again. A lower value frees the Bluetooth adapter sooner, which matters with `releaseAdapterWhenIdle`; a higher value means fewer reconnects. (Default: 29)

## releaseAdapterWhenIdle

*(since 2.5.0)*

If `true`, the proxy hands the Bluetooth adapter back to the system (BlueZ) whenever no connection to the vehicle is open, so other Bluetooth services on the same host can use it, for example a door lock. By default the proxy keeps the adapter for itself until it stops. With this option every new connection sets up the adapter again, which takes a little longer. Combine it with a lower `connectionTimeout` (for example `10`) to free the adapter sooner after a command. (Default: false)

# Example

## Docker compose
You can set the environment variables in your docker-compose.yml file. Example:

```
environment:
  - logLevel=debug
  - scanTimeout=5
  - cacheMaxAge=10
  - vehicleDataCacheTime=60
  - httpListenAddress=:5687
```

This will set the log level to debug, the scanTimeout to 5 seconds, the HTTP cache max age to 10 seconds, the VehicleData cache time to 60 seconds, and the HTTP listen address to :5687.

## Command line

You can also set the environment variables in the command line when starting the program. Example:

```
logLevel=debug scanTimeout=5 cacheMaxAge=10 vehicleDataCacheTime=60 httpListenAddress=:5687 ./TeslaBleHttpProxy
```
