# TeslaBleHttpProxy

TeslaBleHttpProxy is a program written in Go that receives HTTP requests and forwards them via Bluetooth to a Tesla vehicle. The program can, for example, be easily used together with [evcc](https://github.com/evcc-io/evcc).

The program stores the received requests in a queue and processes them one by one. This ensures that only one Bluetooth connection to the vehicle is established at a time.

## Table of Contents

- [How to install](#how-to-install)
  - [Docker compose](#docker-compose)
  - [Build yourself](#build-yourself)
- [Generate key for vehicle](#generate-key-for-vehicle)
- [Setup EVCC](#setup-evcc)
- [API](#api)

## How to install

You can either compile and use the Go program yourself or install it in a Docker container. ([detailed instruction](docs/installation.md))

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

Please remember to create an empty folder where the keys can be stored later. In this example, it is `~/TeslaBleHttpProxy/key`.

Pull and start TeslaBleHttpProxy with `docker compose up -d`.

### Build yourself

Download the code and save it in a folder named 'TeslaBleHttpProxy'. From there, you can easily compile the program.

```
go build .
./TeslaBleHttpProxy
```

Please remember to create an empty folder called `key` where the keys can be stored later.

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

***Since version 0.128.0 or newer of evcc it is very easy to integrate ble proxy:***
```
- name: model3
    type: template
    template: tesla
    title: Tesla
    icon: car
    accessToken: YOUR_ACCESS_TOKEN
    refreshToken: YOUR_REFRSH_TOKEN
    capacity: 60
    vin: YOUR_VIN
    commandProxy: http://YOUR_IP:8080
```

If you want to use an older version:


***Attention: You have to use at least version 0.127.2 or newer of evcc***
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
- set_charge_limit
- charge_port_door_open
- charge_port_door_close
- flash_lights





