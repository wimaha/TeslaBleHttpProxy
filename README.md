# TeslaBleHttpProxy

TeslaBleHttpProxy is a program written in Go that receives HTTP requests and forwards them via Bluetooth to a Tesla vehicle. The program can, for example, be easily used together with [evcc](https://github.com/evcc-io/evcc) or [TeslaBle2Mqtt](https://github.com/Lenart12/TeslaBle2Mqtt).

The program stores the received requests in a queue and processes them one by one. This ensures that only one Bluetooth connection to the vehicle is established at a time.

## Table of Contents

- [How to install](#how-to-install)
  - [Home assistant addon](#home-assistant-addon)
  - [Docker compose](#docker-compose)
  - [Build yourself](#build-yourself)
- [Generate key for vehicle](#generate-key-for-vehicle)
- [Setup EVCC](#setup-evcc)
- [API](#api)
  - [Vehicle Commands](#vehicle-commands)
  - [Vehicle Data](#vehicle-data)
  - [Body Controller State](#body-controller-state)

## How to install

You can either compile and use the Go program yourself or install it as a Home assistant addon or in a Docker container. ([detailed instruction](docs/installation.md))

### Home assistant addon

This proxy is availabile in the [TeslaBle2Mqtt-addon](https://github.com/Lenart12/TeslaBle2Mqtt-addon) repository, included as part of `TeslaBle2Mqtt` addon or as a standalone `TeslaBleHttpProxy` addon.

[![Open your Home Assistant instance and show the add add-on repository dialog with a specific repository URL pre-filled.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https://github.com/Lenart12/TeslaBle2Mqtt-addon)


### Docker compose

Below you will find the necessary contents for your `docker-compose.yml`:

```
services:
  tesla-ble-http-proxy:
    image: wimaha/tesla-ble-http-proxy
    container_name: tesla-ble-http-proxy
    environment:
      - cacheMaxAge=5 # Optional, but recommended to set this to anything more than 0 if you are using the vehicle data
    volumes:
      - ~/TeslaBleHttpProxy/key:/key
      - /var/run/dbus:/var/run/dbus
    restart: always
    privileged: true
    network_mode: host
    cap_add:
      - NET_ADMIN
      - SYS_ADMIN
```

Please remember to create an empty folder where the keys can be stored later. In this example, it is `~/TeslaBleHttpProxy/key`.

Pull and start TeslaBleHttpProxy with `docker compose up -d`.

Note that you can optionally set environment variables to override the default behavior. See [environment variables](docs/environment_variables.md) for more information.

### Build yourself

Download the code and save it in a folder named 'TeslaBleHttpProxy'. From there, you can easily compile the program.

```
go build .
./TeslaBleHttpProxy -h
usage: TeslaBleHttpProxy [-h|--help] [-l|--logLevel "<value>"]
                         [-b|--httpListenAddress "<value>"] [-s|--scanTimeout
                         <integer>] [-c|--cacheMaxAge <integer>] [-k|--keys
                         "<value>"] [-d|--dashboardBaseUrl "<value>"]
                         [-a|--apiBaseUrl "<value>"] [-B|--btAdapter "<value>"]

                         Proxy for Tesla BLE commands over HTTP

Arguments:

  -h  --help               Print help information
  -l  --logLevel           Log level (DEBUG, INFO, WARN, ERROR, FATAL).
                           Default: INFO
  -b  --httpListenAddress  HTTP bind address. Default: :8080
  -s  --scanTimeout        Time in seconds to scan for BLE beacons during
                           device scan (0 = max). Default: 1
  -c  --cacheMaxAge        Time in seconds for Cache-Control header (0 = no
                           cache). Default: 5
  -k  --keys               Path to public and private keys. Default: key
  -d  --dashboardBaseUrl   Base URL for dashboard (Useful if the proxy is
                           behind a reverse proxy). Default: 
  -a  --apiBaseUrl         Base URL for proxying API commands. Default: 
  -B  --btAdapter          Bluetooth adapter ID to use ("hciX"). Default:
                           Default adapter
```

Please remember to create an empty folder called `key` where the keys can be stored later.

Note that you can optionally set environment variables to override the default behavior. See [environment variables](docs/environment_variables.md) for more information.

## Generate key for vehicle

*(Here, the simple, automatic method is described. Besides the automatic method, you can also generate the keys [manually](docs/manually_gen_key.md).)*

To generate the required keys browse to `http://YOUR_IP:8080/dashboard`. In the dashboard you will see that the keys are missing:

<img src="docs/proxy1.png" alt="Picture of the Dashboard with missing keys." width="40%" height="40%">

Please click on `generate Keys` and the keys will be automatically generated and saved.

<img src="docs/proxy2.png" alt="Picture of the Dashboard with success message and keys." width="40%" height="40%">

After that please enter your VIN under `Setup Vehicle`. Before you proceed make sure your vehicle is awake! So you have to manually wake the vehicle before you send the key to the vehicle.

<img src="docs/proxy3.png" alt="Picture of Setup Vehicle Part of the Dashboard." width="40%" height="40%">

Finally the keys is send to the vehicle. You have to confirm by tapping your NFC card on center console.

<img src="docs/proxy6.png" alt="Picture of success message sent add-key request." width="40%" height="40%">

You can now close the dashboard and use the proxy. 🙂

## Setup EVCC

You can use the following configuration in evcc (recommended):

```
vehicles:
  - name: tesla
    type: template
    template: tesla-ble
    title: Your Tesla (optional)
    capacity: 60 # Akkukapazität in kWh (optional)
    vin: VIN # Erforderlich für BLE-Verbindung
    url: IP # URL des Tesla BLE HTTP Proxy
    port: 8080 # Port des Tesla BLE HTTP Proxy (optional)
```

If you want to use this proxy only for commands, and not for vehicle data, you can use the following configuration. The vehicle data is then fetched via the Tesla API by evcc.

```
- name: model3
    type: template
    template: tesla
    title: Tesla
    icon: car
    commandProxy: http://YOUR_IP:8080
    accessToken: YOUR_ACCESS_TOKEN
    refreshToken: YOUR_REFRSH_TOKEN
    capacity: 60
    vin: YOUR_VIN
```

(Hint for multiple vehicle support: https://github.com/wimaha/TeslaBleHttpProxy/issues/40)

## API

### Vehicle Commands

The program uses the same interfaces as the Tesla [Fleet API](https://developer.tesla.com/docs/fleet-api#vehicle-commands). Currently, most commands are supported.

By default, the program will return immediately after sending the command to the vehicle. If you want to wait for the command to complete, you can set the `wait` parameter to `true` (`charge_start?wait=true`).

#### Example Request

*(All requests with method POST.)*

Start charging:
`http://localhost:8080/api/1/vehicles/{VIN}/command/charge_start`

Start charging and wait for the command to complete:
`http://localhost:8080/api/1/vehicles/{VIN}/command/charge_start?wait=true`

Stop charging:
`http://localhost:8080/api/1/vehicles/{VIN}/command/charge_stop`

Set charging amps to 5A:
`http://localhost:8080/api/1/vehicles/{VIN}/command/set_charging_amps` with body `{"charging_amps": "5"}`

### Vehicle Data

The vehicle data is fetched from the vehicle and returned in the response in the same format as the [Fleet API](https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-endpoints#vehicle-data). Since a ble connection has to be established to fetch the data, it takes a few seconds before the data is returned.

#### Example Request

*(All requests with method GET.)*

Get vehicle data:
`http://localhost:8080/api/1/vehicles/{VIN}/vehicle_data`

Currently you will receive the following data:

- charge_state
- climate_state

If you want to receive specific data, you can add the endpoints to the request. For example:

`http://localhost:8080/api/1/vehicles/{VIN}/vehicle_data?endpoints=charge_state`

This is recommended if you want to receive data frequently, since it will reduce the time it takes to receive the data.

All of the supported endpoints are:
- charge_schedule_data
- charge_state
- climate_state
- closures_state
- drive_state
- location_data
- media_detail
- media
- parental_controls
- preconditioning_schedule_data
- software_update
- tire_pressure

### Body Controller State

The body controller state is fetched from the vehicle and returnes the state of the body controller. The request does not wake up the vehicle. The following information is returned:
- `closure_statuses`	
  - `charge_port`
    - `CLOSURESTATE_CLOSED`
    - `CLOSURESTATE_OPEN`
    - `CLOSURESTATE_AJAR`
    - `CLOSURESTATE_UNKNOWN`
    - `CLOSURESTATE_FAILED_UNLATCH`
    - `CLOSURESTATE_OPENING`
    - `CLOSURESTATE_CLOSING`
  - `front_driver_door`
    - ...
  - `front_passenger_door`
    - ...
  - `front_trunk`
    - ...
  - `rear_driver_door`
    - ...
  - `rear_passenger_door`
    - ...
  - `rear_trunk`
    - ...
  - `tonneau`
    - ...
- `vehicle_lock_state`
  - `VEHICLELOCKSTATE_UNLOCKED`
  - `VEHICLELOCKSTATE_LOCKED`
  - `VEHICLELOCKSTATE_INTERNAL_LOCKED`
  - `VEHICLELOCKSTATE_SELECTIVE_UNLOCKED`
- `vehicle_sleep_status`
  - `VEHICLE_SLEEP_STATUS_UNKNOWN`
  - `VEHICLE_SLEEP_STATUS_AWAKE`
  - `VEHICLE_SLEEP_STATUS_ASLEEP`
- `user_presence`
  - `VEHICLE_USER_PRESENCE_UNKNOWN`
  - `VEHICLE_USER_PRESENCE_NOT_PRESENT`
  - `VEHICLE_USER_PRESENCE_PRESENT`

#### Request

*(All requests with method GET.)*

Get body controller state:
`http://localhost:8080/api/proxy/1/vehicles/{VIN}/body_controller_state`

### Connection status

Get BLE connection status of the vehicle
`GET http://localhost:8080/api/proxy/1/vehicles/{VIN}/connection_status`
- `address`
- `connectable`
- `local_name`
- `operated`
- `rssi`