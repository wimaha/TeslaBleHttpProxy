## Manually generate key for vehicle

Besides the automatic method, you can also generate the keys manually.
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
