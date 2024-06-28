# Installation

## Step 1: Install Raspberian

Setup your raspberry Pi. You will find an installer here: https://www.raspberrypi.com/software/

Login to your Pi with a terminal. 

```
ssh user@IP
```

Update everything once after installation:

```
sudo apt-get update && sudo apt-get upgrade
```

## Step 2: Generate key for vehicle

You can generate the required keys and send them to your tesla using the following steps. Be sure you are in the home directory `~`.

Download and install Go:
```
sudo apt update && sudo apt install -y wget git build-essential
wget https://dl.google.com/go/go1.22.1.linux-arm64.tar.gz
tar -xvf go1.22.1.linux-arm64.tar.gz
mkdir -p ~/.local/share && mv go ~/.local/share
export GOPATH=$HOME/.local/share/go
export PATH=$HOME/.local/share/go/bin:$PATH
echo 'export GOPATH=$HOME/.local/share/go' >> ~/.bashrc
echo 'export PATH=$HOME/.local/share/go/bin:$PATH' >> ~/.bashrc
```

Download and install tesla vehicle-command:
```
git clone https://github.com/teslamotors/vehicle-command.git
cd vehicle-command
go get ./...
go build ./...
go install ./...
sudo setcap 'cap_net_admin=eip' "$(which tesla-control)"
```

Generate a private and a public key
```
openssl ecparam -genkey -name prime256v1 -noout > private.pem
openssl ec -in private.pem -pubout > public.pem
```

Send the puclic key via BLE to your tesla. Be sure to replace YOUR_VIN with your VIN.
```
tesla-control -vin YOUR_VIN -ble add-key-request public.pem owner cloud_key
```
After you have successfully triggered the last command, you must tap the key card in the center console (no message is displayed on the Tesla before tapping the card) and confirm the addition of the key.

## Step 3: Install TeslaBleHttpProxy

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
```

Create the docker compose file and open it:

```
nano docker-compose.yml
```

Paste the following content to the file:

```
version: '3.1'
services:
  tesla-ble-http-proxy:
    image: wimaha/tesla-ble-http-proxy
    container_name: tesla-ble-http-proxy
    volumes:
      - ~/vehicle-command:/key
      - /var/run/dbus:/var/run/dbus
    restart: always
    privileged: true
    network_mode: host
    cap_add:
      - NET_ADMIN
      - SYS_ADMIN
```

Exit the file with control + x and type `y` to save the file.

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
