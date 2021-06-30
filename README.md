This is simple Smart [HomeKit](https://developer.apple.com/homekit/) appliance daemon written in [GoLang](https://golang.org) for embedding controls in smart home systems.

# Requirements

* RaspberryPI or Compatible (OrangePI, BananaPI) (golang supported architecture) SoC running Linux with wiringpi support.

# Main advantage over other solutions

* Does not consume resources
* Works on almost every PI based board
* No hard or bloated dependencies
* Does not require any scripting language
* Cross-platform
* Stability!

# Supported sensors/devices

* [DS18B20](https://www.aliexpress.com/item/4000895660165.html) Waterproof temperature probe
* [DHT22](https://www.aliexpress.com/item/1005001621864387.html) Digital Temperature and Humidity Sensors
* [Sonoff R3 Basic](https://www.aliexpress.com/item/33046496288.html)
* GPIO connected [relay boards](https://www.aliexpress.com/item/1005002806849848.html) (almost all)

# Screens

<img src="https://github.com/e1z0/Go_HomeKit_Relay/blob/ef7c86dd01bee400f2ea60a51a99ccdc61d66324/pics/IMG_3415.png" width=30% height=30%><img src="https://github.com/e1z0/Go_HomeKit_Relay/blob/ef7c86dd01bee400f2ea60a51a99ccdc61d66324/pics/IMG_3416.png" width=30% height=30%><img src="https://github.com/e1z0/Go_HomeKit_Relay/blob/ef7c86dd01bee400f2ea60a51a99ccdc61d66324/pics/IMG_3417.png" width=30% height=30%><img src="https://github.com/e1z0/Go_HomeKit_Relay/blob/ef7c86dd01bee400f2ea60a51a99ccdc61d66324/pics/IMG_3418.png" width=30% height=30%>

# GoLang prepare 

Download and install golang. In /etc/profile set these variables:
```
export LC_ALL=en_US.UTF-8
export PATH=$PATH:/usr/local/go/bin
export GOPATH=~/go
```

## Compile

```
apt-get install wiringpi
make deps
make
```

# TODO

* RTSP Cameras support
* Motion sensors
* Standartized way to support different types of devices and sensors
