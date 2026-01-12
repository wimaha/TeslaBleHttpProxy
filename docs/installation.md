# Installation

## Step 1: Install Raspberry Pi OS

Setup your raspberry Pi. You will find an installer here: https://www.raspberrypi.com/software/

Login to your Pi with a terminal. 

```
ssh user@IP
```

Update everything once after installation:

```
sudo apt-get update && sudo apt-get upgrade
```

## Step 2: Install TeslaBleHttpProxy

There are two alternative ways to install TeslaBleHttpProxy. You can either run the program in Docker or compile it with Go and run it directly.

- [Install with Docker](#step-a-1-install-docker) or
- [Compile and run directly](#step-b-1-download-and-build)

*(You must either follow steps A-x or steps B-x. You do not have to do both!)*

### Step A-1: Install Docker

Go back to your home directory:

```
cd ~
```

Install Docker:

```
curl -sSL https://get.docker.com | sh
```

Setup the Docker user:

```
sudo usermod -aG docker $USER
```

Now you have to log out and log back in for it to take effect.

### Step A-2: Setup docker compose

Make sure you are in the home directory `~` again. Create a Folder for your Docker-Files for example `TeslaBleHttpProxy` and enter the new folder:

```
mkdir TeslaBleHttpProxy
cd TeslaBleHttpProxy
mkdir key
```

Create the docker compose file and open it:

```
nano docker-compose.yml
```

Paste the following content to the file:

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

Exit the file with control + x and type `y` to save the file.

Note that you can optionally set environment variables to override the default behavior. See [environment variables](docs/environment_variables.md) for more information.

### Step A-3: Start the container

Start the container with the following command:

```
docker compose up -d
```

### Step A-4: Update and Show logs

You can update the container as follows:

```
docker pull wimaha/tesla-ble-http-proxy
docker compose up -d
```

You can show the logs like:

```
docker logs --since 12h tesla-ble-http-proxy
```

### Step B-1: Download and Build

This variant will be described later.

## Step 3: Generate key for vehicle

**Security Recommendation:** We recommend using the **Charging Manager** role for security. It provides limited access suitable for charging management and works perfectly with [evcc tesla-ble template](https://docs.evcc.io/docs/devices/vehicles#tesla-ble). The Charging Manager role can:
- Read vehicle data
- Authorize charging-related commands: `wake`, `charge_start`, `charge_stop`, `set_charging_amps`

The **Owner** role provides full access to all vehicle functions (unlock, start, etc.) and should only be used if you need non-charging functions.

To generate the required keys browse to `http://YOUR_IP:8080/dashboard`. In the dashboard you will see that the keys are missing:

<img src="proxy1.png" alt="Picture of the Dashboard with missing keys." width="40%" height="40%" style="box-shadow: 0 0 10px rgba(0, 0, 0, 0.1); margin-bottom: 10px;">

Please click on `Generate` for the **Charging Manager** role (recommended for security). The keys will be automatically generated and saved. The Charging Manager key will be set as active by default.

<img src="proxy2b.png" alt="Picture of the Dashboard with success message and keys." width="40%" height="40%"><br/>
<img src="proxy2.png" alt="Picture of the Dashboard with success message and keys." width="40%" height="40%" style="box-shadow: 0 0 10px rgba(0, 0, 0, 0.1); margin-bottom: 10px;">

After that please enter your VIN under `Setup Vehicle`. Before you proceed make sure your vehicle is awake! So you have to manually wake the vehicle before you send the key to the vehicle.

<img src="proxy3.png" alt="Picture of Setup Vehicle Part of the Dashboard." width="40%" height="40%" style="box-shadow: 0 0 10px rgba(0, 0, 0, 0.1); margin-bottom: 10px;">

The key is now sent to the vehicle. To complete the process, confirm by tapping your NFC card on the center console. (Note: There will be no message on the Tesla screen before confirmation with the NFC card.)

<img src="proxy6.png" alt="Picture of success message sent add-key request." width="40%" height="40%">

You can now close the dashboard and use the proxy. 🙂
