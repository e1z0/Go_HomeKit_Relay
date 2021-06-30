This is simple GoLang written smart homekit appliance daemon for embedding controls in smart home systems.

# Requirements

* RaspberryPI or Compatbile (OrangePI, BananaPI) arm/arm64 SoC running Linux

# Supported sensors/devices

* (DS18B20)[https://www.aliexpress.com/item/4000895660165.html] Waterproof temperature probe
* (DHT22)[https://www.aliexpress.com/item/1005001621864387.html] Digital Temperature and Humidity Sensors
* (Sonoff R3 Basic)[https://www.aliexpress.com/item/33046496288.html]
* GPIO connected (relay boards)[https://www.aliexpress.com/item/1005002806849848.html] (almost all)

# Screens

[](/pics/IMG_3415.png)
[](/pics/IMG_3416.png)
[](/pics/IMG_3417.png)
[](/pics/IMG_3418.png)

# GoLang prepare 

Download and install golang
In /etc/profile set these variables:
export LC_ALL=en_US.UTF-8
export PATH=$PATH:/usr/local/go/bin
export GOPATH=~/go

## Compile

apt-get install wiringpi
make deps
make

# TODO

* RTSP Cameras support

