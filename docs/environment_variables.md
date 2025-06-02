# Environment variables

You can optionally set environment variables to override the default behavior.

## logLevel

This is the log level. Options: debug (Default: info)

## scanTimeout

This is the number of seconds to scan for BLE devices. If set to 0, the scan will continue until a device is found or the context is cancelled. (Default: 2)

## cacheMaxAge

This is the number of seconds to cache the BLE responses for vehicle data and body controller state. If set to 0, the cache is disabled. (Default: 5)

## httpListenAddress

This is the address and port to listen for HTTP requests. (Default: :8080)

# Example

## Docker compose
You can set the environment variables in your docker-compose.yml file. Example:

```
environment:
  - logLevel=debug
  - scanTimeout=5
  - cacheMaxAge=10
  - httpListenAddress=:5687
```

This will set the log level to debug, the scanTimeout to 5 seconds, the cache max age to 10 seconds, and the HTTP listen address to :5687.

## Command line

You can also set the environment variables in the command line when starting the program. Example:

```
logLevel=debug scanTimeout=5 cacheMaxAge=10 httpListenAddress=:5687 ./TeslaBleHttpProxy
```
