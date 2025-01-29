# Environment variables

You can optionally set environment variables to override the default behavior.

## logLevel

This is the log level. Options: debug (Default: INFO)

## cacheMaxAge

This is the value that will be set in Cache-Control header for vehicle data and body controller state responses. If set to 0, the cache is disabled. (Default: 5)

## httpListenAddress

This is the address and port to listen for HTTP requests. (Default: :8080)

# Example

## Docker compose
You can set the environment variables in your docker-compose.yml file. Example:

```
environment:
  - logLevel=debug
  - cacheMaxAge=30
  - httpListenAddress=:5687
```

This will set the log level to debug, the cache max age to 30 seconds, and the HTTP listen address to :5687.

## Command line

You can also set the environment variables in the command line when starting the program. Example:

```
./TeslaBleHttpProxy --logLevel=debug --cacheMaxAge=30 --httpListenAddress=:5687
```

## Caution

> [!WARNING]
> If you set both environment variables and command line options for the same setting, you will see the error `[command] can only be present once`
