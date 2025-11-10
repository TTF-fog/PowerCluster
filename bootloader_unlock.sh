#made for Arch Linux
# I AHTE BASH  I HATE BASH I HATE BASH
sudo pacman -S --needed android-tools #adb
sudo pacman -S --needed usbutils      #lsusb
devices=(
  "Samsung"
  "OnePlus"
  "Realtek"
)

usb_output=$(lsusb)
found=false

for dev in "${devices[@]}"; do
  if echo "$usb_output" | grep -qi "$dev"; then
    echo "supported device found ${dev}"
    found=true
  fi
done

if ! $found; then
  echo "fatal: no devices"
fi
devices=$(adb devices | grep -w "device" | grep -v "List")
if [ -z "$devices" ]; then
  echo "cant find adb devices, check if ADB IS enabled"
  exit 1
fi
adb devices
adb kill-server && adb start-server
echo "Serial Number:"
read ANDROID_SERIAL
echo "continue to bootloader reset (y/N)"
read continue
if ["${continue}" == "y"]; then
  adb "fastboot oem unlock"
  echo "done"
  exit
fi
echo "quit ,exiting"
exit 1
