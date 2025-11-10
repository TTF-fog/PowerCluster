#https://drive.google.com/file/d/1pQ5A0yJDBMO92uh4-qROcdpHTemuzKGx/view
# !!! YOU NEED THIS KERNEL ON THE PHONE !!
# !!! FLASH IT YOURSELF!!!
# Credit: Denys Vitali for the kernel
string= ip addr show wlan0
if [[ $string == *"does not exist"* ]]; then
  echo "wifi borked exiting"
  exit 1
fi
echo "first 10 networks"
nmcli d wifi | head -n 5
echo "ssid:"
read ssid
nmcli d wifi connect "${ssid}"--ask
apk add k3s

sudo wget "https://192.168.0.1:8080/k3s_config" --output-file /etc/conf.d/k3s
sudo wget "https://192.168.0.1:8080/rancher_config" --output-file /etc/rancher/k3s/config.yaml
service k3s start
rc-update add k3s
