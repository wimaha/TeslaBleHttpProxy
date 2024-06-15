# TeslaBleHttpProxy

TeslaBleHttpProxy is a program written in Go that receives HTTP requests and forwards them via Bluetooth to a Tesla vehicle. The program can, for example, be easily used together with [evcc](https://github.com/evcc-io/evcc).

The program stores the received requests in a queue and processes them one by one. This ensures that only one Bluetooth connection to the vehicle is established at a time.

## Table of Contents

- [How to install](#how-to-install)
  - [Build yourself](#build-yourself)
  - [Docker compose](#docker-compose)
- [Generate key for vehicle](#generate-key-for-vehicle)
- [Setup EVCC](#setup-evcc)
- [API](#api)

## How to install

You can either compile and use the Go program yourself or install it in a Docker container.

### Build yourself

Download the code and save it in a folder named 'TeslaBleHttpProxy'. From there, you can easily compile the program.

```
go build .
./TeslaBleHttpProxy
```

Be sure to add the `private.pem` in a folder called `key`.

### Docker compose

Below you will find the necessary contents for your `docker-compose.yml`:

```
services:
  tesla-ble-http-proxy:
    image: wimaha/tesla-ble-http-proxy
    container_name: tesla-ble-http-proxy
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

Please ensure that you specify the folder containing the `private.pem` correctly. In this example, it is `~/TeslaBleHttpProxy/key`.

## Generate key for vehicle

You can generate the required keys using the following steps. For more information, see also the documentation of the [Tesla Vehicle Command SDK](https://github.com/teslamotors/vehicle-command/blob/main/cmd/tesla-control/README.md).

```
git clone https://github.com/teslamotors/vehicle-command.git
cd vehicle-command
go get ./...
go build ./...
go install ./...
openssl ecparam -genkey -name prime256v1 -noout > private.pem
openssl ec -in private.pem -pubout > public.pem
sudo setcap 'cap_net_admin=eip' "$(which tesla-control)"
tesla-control -vin YOUR_VIN -ble add-key-request public.pem owner cloud_key
```

## Setup EVCC

***Attention: Currently the custom vehicle config only works on evcc version 0.126.6 (`image: evcc/evcc:0.126.6`)***
Below is a sample configuration of a custom vehicle in evcc:

```
vehicles:
  - name: model3
    type: custom
    title: Tesla Model 3
    capacity: 60
    chargeenable:
      source: http
      uri: "http://IP:8080/api/1/vehicles/VIN/command/{{if .chargeenable}}charge_start{{else}}charge_stop{{end}}"
      method: POST
      body: ""
    maxcurrent: # set charger max current (A)
      source: http
      uri: http://IP:8080/api/1/vehicles/VIN/command/set_charging_amps
      method: POST
      body: '{"charging_amps": "{{.maxcurrent}}"}'
    wakeup: # vehicle wake up command
      source: http
      uri: http://IP:8080/api/1/vehicles/VIN/command/wake_up
      method: POST
      body: ""
    soc:
      source: [Your Source ...]
    range:
      source: [Your Source ...]
    status:
      source: combined
      plugged:
        source: [Your Source ...]
      charging:
        source: [Your Source ...]
```

## API

The program uses the same interfaces as the Tesla [Fleet API](https://developer.tesla.com/docs/fleet-api#vehicle-commands). Currently, the following requests are supported: 

- wake_up
- charge_start
- charge_stop
- set_charging_amps
- flash_lights





