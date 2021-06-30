/*
Copyright (c) 2019-2021 e1z0
EofNET
*/

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/service"
	. "github.com/cyoung/rpi"
	"github.com/d2r2/go-dht"
)

var (
	settings Settings
	DEBUG    bool
	Version  string
	Build    string
	Commit   string
)

type RelayDevice struct {
	Id     int
	Type   int // 0 - lamp, 1 - switch, 2 - cooler
	Name   string
	Pin    int
	Invert int
}

func GetRelayById(id int) RelayDevice {
	var dev RelayDevice
	for _, key := range settings.Relay_pins {
		if key.Id == id {
			return key
		}
	}
	return dev
}

type Settings struct {
	Model                   string
	Name                    string
	Pin                     string
	Relays_enabled          bool
	Relay_pins              []RelayDevice
	Dht22_enabled           bool
	Dht22_name              string
	Dht22_pin               int
	Dht22_vcc_pin           int
	Dht22_autorecovery      bool
	Dht22_update_tickness   int
	Ds18b20_enabled         bool
	Ds18b20_ids             map[string]string
	Ds18b20_data_pin        int
	Ds18b20_vcc_pin         int
	Ds18b20_autorecovery    bool
	Ds18b20_updateinterval  int
	Sonoff_r3_basic_enabled bool
	Sonoff_r3_basic_devices map[string]string
	Camera_enabled          bool
	Cemera_device           string
	Debug                   bool
}

func ProgramPanic(err error) {
	log.Printf("Program panic: %s\n", err)
	panic(err)
}

func ReadSettings() {
	settingsfile := "settings.json"
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		ProgramPanic(err)
	}
	if DEBUG {
		log.Printf("Program is running from: %s directory\n", dir)
	}

	jsonFile, err := os.Open(dir + "/" + settingsfile)
	// if we os.Open returns an error then handle it
	if err != nil {
		log.Printf("We detected that the configuration file was not found on this system!\n")
		log.Printf("We will create new one for you :)\n")
		file, _ := json.Marshal(settings)
		err = ioutil.WriteFile(settingsfile, file, 0644)
		if err != nil {
			ProgramPanic(err)
		}
		log.Printf("We have sucessfully created settings file %s in program's directory!\n", settingsfile)
		log.Printf("Edit the configuration and run this program again :)\n")
		os.Exit(1)
	}
	if DEBUG {
		fmt.Printf("Successfully Opened %s settings file\n", settingsfile)
	}
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Println(err)
	}
	json.Unmarshal(byteValue, &settings)
	// turn on the debug settings
	DEBUG = false
	if settings.Debug {
		DEBUG = true
		log.Printf("!!! WARNING !!! ACHTUNGT !!!! The Developer debug logging is turned on !!!!\n")
	}
	if DEBUG {
		log.Printf("Settings loaded!\n")
		log.Printf("%+v\n", settings)
	}
	return

}

func ReadTemp(pin int) (float64, float64) {
	temperature, humidity, retried, err := dht.ReadDHTxxWithRetry(dht.DHT22, pin, false, 10)
	if err != nil {
		if DEBUG {
			log.Printf("Failed to read temperature from the dth22 sensor on pin: %d, maybe the ping is wrong? %s\n", pin, err)
			if settings.Dht22_autorecovery {
				RecoveryPin(settings.Dht22_vcc_pin)
			}
			return 0, 0
		}
	}
	// Print temperature and humidity
	if DEBUG {
		log.Printf("Temperature = %v*C, Humidity = %v%% (retried %d times)\n", temperature, humidity, retried)
	}
	return float64(temperature), float64(humidity)
}

func SwitchState(rawpin string) bool {
	pin, err := strconv.Atoi(rawpin)
	if err == nil {
		if DEBUG {
			log.Printf("Reading state for the pin %d\n", pin)
		}
		state := DigitalRead(pin)
		fmt.Printf("Read value was %d\n", state)
		if state == 0 {
			return true
		}
	}
	return false
}

func OnSwitch(rawpin string) {
	pin, err := strconv.Atoi(rawpin)
	if err == nil {
		if DEBUG {
			log.Printf("Turning power on for switch %d\n", pin)
		}
		PinMode(pin, OUTPUT)
		DigitalWrite(pin, LOW)
	}
}

func OffSwitch(rawpin string) {
	pin, err := strconv.Atoi(rawpin)
	if err == nil {
		if DEBUG {
			log.Printf("Turning power off for switch %d\n", pin)
		}
		PinMode(pin, OUTPUT)
		DigitalWrite(pin, HIGH)
	}
}

type SonoffV3 struct {
	DeviceId string
	DeviceIp string
}

type SonoffInfo struct {
	Seq   int `json:"seq"`
	Error int `json:"error"`
	Data  struct {
		Switch         string `json:"switch"`
		Startup        string `json:"startup"`
		Pulse          string `json:"pulse"`
		PulseWidth     int    `json:"pulseWidth"`
		Ssid           string `json:"ssid"`
		OtaUnlock      bool   `json:"otaUnlock"`
		FwVersion      string `json:"fwVersion"`
		DeviceId       string `json:"deviceid"`
		Bssid          string `json:"bssid"`
		SignalStrength int    `json:"signalstrength"`
	} `json:"data"`
}

type SonoffPostData struct {
	DeviceId string `json:"deviceid"`
	Data     struct {
		Switch string `json:"switch"`
	} `json:"data"`
}

func SonoffGetProps(ip string) SonoffV3 {
	sondev := SonoffV3{}
	data := SonoffPostData{}
	jsonData, err := json.Marshal(&data)
	if err != nil {
		fmt.Printf("Got problems in marshalling the json data for sonoff switch2\n")
		return sondev
	}
	var jsonStr = []byte(jsonData)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s:8081/zeroconf/info", ip), bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return sondev
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	// we should umarshall the output of the sonoff info
	soninfo := SonoffInfo{}
	err = json.Unmarshal(body, &soninfo)
	if err != nil {
		fmt.Printf("Problems in umarshalling the sonoff info output! %s\n", err)
	}
	sondev = SonoffV3{DeviceId: soninfo.Data.DeviceId, DeviceIp: ip}
	return sondev
}

func SonoffOn(ip string) bool {
	sondev := SonoffGetProps(ip)
	data := SonoffPostData{DeviceId: sondev.DeviceId}
	data.Data.Switch = "on"
	jsonData, err := json.Marshal(&data)
	if err != nil {
		fmt.Printf("Got problems in marshalling the json data for sonoff switch\n")
		return false
	}
	var jsonStr = []byte(jsonData)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s:8081/zeroconf/switch", sondev.DeviceIp), bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return true
}

func SonoffOff(ip string) bool {
	sondev := SonoffGetProps(ip)
	data := SonoffPostData{DeviceId: sondev.DeviceId}
	data.Data.Switch = "off"
	jsonData, err := json.Marshal(&data)
	if err != nil {
		fmt.Printf("Got problems in marshalling the json data for sonoff switch\n")
		return false
	}
	var jsonStr = []byte(jsonData)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s:8081/zeroconf/switch", sondev.DeviceIp), bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return true
}

func SonoffGetState(ip string) bool {
	sondev := SonoffGetProps(ip)
	data := SonoffPostData{DeviceId: sondev.DeviceId}
	jsonData, err := json.Marshal(&data)
	if err != nil {
		fmt.Printf("Got problems in marshalling the json data for sonoff switch\n")
		return false
	}
	var jsonStr = []byte(jsonData)
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s:8081/zeroconf/info", sondev.DeviceIp), bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	// we should umarshall the output of the sonoff info
	soninfo := SonoffInfo{}
	err = json.Unmarshal(body, &soninfo)
	if err != nil {
		fmt.Printf("Problems in umarshalling the sonoff info output! %s\n", err)
	}
	if soninfo.Data.Switch == "on" {
		return true
	}
	return false

}

// it needs to be passed the phisical pin id to this function, not the gpio or other shit
func RecoveryPin(pin int) {
	log.Printf("Powering off pin %d\n", pin)
	PinMode(BoardToPin(pin), OUTPUT)
	DigitalWrite(BoardToPin(pin), LOW)
	log.Printf("Waiting 10 secondds...\n")
	time.Sleep(time.Second * 10)
	PinMode(pin, OUTPUT)
	DigitalWrite(BoardToPin(pin), HIGH)
	log.Printf("Powering on pin %d\n", pin)
	time.Sleep(time.Second * 10)
	return
}

func main() {
	log.Printf("Welcome to IOT HOME Agent v%s built: %s commit: %s\n", Version, Build, Commit)
	log.Printf("Copyright (c) 2018-2020 EofNET Networks\n\n\n")
	ReadSettings()
	log.Printf("Initializing wiring-pi interface...\n")
	WiringPiSetup()
	var accessories []*accessory.Accessory

	if settings.Relays_enabled {
		log.Printf("Loading relay support..\n")
		log.Printf("Sorting relays by id..\n")
		sort.SliceStable(settings.Relay_pins, func(i, j int) bool {
			return settings.Relay_pins[i].Id < settings.Relay_pins[j].Id
		})
		log.Printf("Sort done..\n")
		for _, key := range settings.Relay_pins {
			log.Printf("Found relay switch %#v\n", key)
			// setting up the component depending on the type
			if key.Type == 0 {
				component := *accessory.NewLightbulb(accessory.Info{Name: key.Name, Manufacturer: "EofNET", SerialNumber: fmt.Sprintf("%d", key.Pin)})
				if SwitchState(component.Info.SerialNumber.GetValue()) == true {
					component.Lightbulb.On.SetValue(true)
				}
				component.Lightbulb.On.OnValueRemoteUpdate(func(on bool) {
					if on == true {
						OnSwitch(component.Info.SerialNumber.GetValue())

					} else {
						OffSwitch(component.Info.SerialNumber.GetValue())
					}
				})
				accessories = append(accessories, component.Accessory)
			}
			if key.Type == 1 {
				component := accessory.NewSwitch(accessory.Info{Name: key.Name, Manufacturer: "EofNET", SerialNumber: fmt.Sprintf("%d", key.Pin)})
				accessories = append(accessories, component.Accessory)
			}
			if key.Type == 2 {
				component := NewFan(accessory.Info{Name: key.Name, Manufacturer: "EofNET", SerialNumber: fmt.Sprintf("%d", key.Pin)})
				if key.Invert == 1 && SwitchState(component.Info.SerialNumber.GetValue()) == false {
					component.Fan.On.SetValue(true)
				}
				if key.Invert > 0 {
					component.Fan.On.OnValueRemoteUpdate(func(on bool) {
						if on == true {
							//OnSwitch(component.Info.SerialNumber.GetValue())
							OffSwitch(component.Info.SerialNumber.GetValue())
						} else {
							//OffSwitch(component.Info.SerialNumber.GetValue())
							OnSwitch(component.Info.SerialNumber.GetValue())
						}
					})
				} else {
					component.Fan.On.OnValueRemoteUpdate(func(on bool) {
						if on == true {
							//OnSwitch(component.Info.SerialNumber.GetValue())
							OnSwitch(component.Info.SerialNumber.GetValue())
						} else {
							//OffSwitch(component.Info.SerialNumber.GetValue())
							OffSwitch(component.Info.SerialNumber.GetValue())
						}
					})
				}
				accessories = append(accessories, component.Accessory)
			}
		}
	}
	// sonoff r3 basic Diy relays support
	if settings.Sonoff_r3_basic_enabled {
		log.Printf("Loading sonoff relay support..\n")
		/// some sorting right, lol :D
		reles2 := []string{}
		for rele2 := range settings.Sonoff_r3_basic_devices {
			reles2 = append(reles2, rele2)
		}
		sort.Strings(reles2)

		for _, v := range reles2 {
			key := settings.Sonoff_r3_basic_devices[v]
			log.Printf("Found relay switch %s named %s\n", v, key)
			component := accessory.NewLightbulb(accessory.Info{Name: key, Manufacturer: "EofNET", SerialNumber: v})
			component.Lightbulb.On.OnValueRemoteUpdate(func(on bool) {
				if on == true {
					SonoffOn(component.Info.SerialNumber.GetValue())
				} else {
					SonoffOff(component.Info.SerialNumber.GetValue())
				}
			})
			accessories = append(accessories, component.Accessory)
		}
	}

	if settings.Dht22_enabled {
		// temperature sensors
		log.Printf("Loading dht22 sensor support..\n")
		davikl := NewDaviklis(accessory.Info{Name: settings.Dht22_name})
		accessories = append(accessories, davikl.Accessory)
		// auto update block
		ticker := time.NewTicker(time.Second * time.Duration(settings.Dht22_update_tickness))
		go func() {
			for _ = range ticker.C {
				temp, humidity := ReadTemp(settings.Dht22_pin)
				davikl.Humidity.CurrentRelativeHumidity.SetValue(humidity)
				davikl.Temperature.CurrentTemperature.SetValue(temp)
			}
		}()
		// auto update block end

	}

	if settings.Ds18b20_enabled {
		log.Printf("Loading Ds18b20 support...\n")
		dev_count := 0
		Ds18b20_devices := make(map[int]TempDaviklis)
		for id, name := range settings.Ds18b20_ids {
			//             log.Printf("Found Ds18b20 sensor: %s name: %s\n",id,name)
			count := 0
			tries := 3
			for {
				count++
				_, err := os.Stat("/sys/bus/w1/devices/" + id)
				if os.IsNotExist(err) {
					log.Printf("Recovering Ds18b20 sensors, because it was not found on the system\n")
					RecoveryPin(settings.Ds18b20_vcc_pin)
				} else {
					break
				}
				if count > tries {
					log.Printf("Tried %d times to recover the sensor: %s without any success\n", count, id)
					break
				}
			}
			_, err := os.Stat("/sys/bus/w1/devices/" + id)
			if os.IsNotExist(err) {
				//RecoverDs18b20()
				log.Printf("We were unable to recover the Ds18b20 sensors, try to recover manually and restart the program\n")
			} else {
				// we should add all sensors to map
				// read the sensor data and create the new object for it
				t, err := Ds18b20Temp(id)
				if err == nil {
					log.Printf("Found sensor: %s (%s) temperature: %.2f°C\n", name, id, t)
					dev_count++
					Ds18b20_devices[dev_count] = *NewTempDaviklis(accessory.Info{Name: name, SerialNumber: id})
					accessories = append(accessories, Ds18b20_devices[dev_count].Accessory)

				}
			}

		}
		// here we should implement the tickness of the sensor
		ticker2 := time.NewTicker(time.Second * time.Duration(settings.Ds18b20_updateinterval))
		go func() {
			for _ = range ticker2.C {
				for i := 1; i <= dev_count; i++ {
					temp, err := Ds18b20Temp(Ds18b20_devices[i].Info.SerialNumber.GetValue())
					if err == nil {
						if DEBUG {
							log.Printf("Timer tick %d for Ds18b20 sensor: %s (%s) got temp %.2fC\n", settings.Ds18b20_updateinterval, Ds18b20_devices[i].Info.Name.GetValue(), Ds18b20_devices[i].Info.SerialNumber.GetValue(), temp)
						}
						Ds18b20_devices[i].Temperature.CurrentTemperature.UpdateValue(temp)
					} else {
						log.Printf("Unable to retrieve the Ds18b20 sensor %s temperature, running recovery mode...\n", Ds18b20_devices[i].Info.SerialNumber.GetValue())
						RecoveryPin(settings.Ds18b20_vcc_pin)
					}
				}
			}
		}()

	}

	config := hc.Config{Pin: settings.Pin, StoragePath: "./db"}
	t, err := hc.NewIPTransport(config, NewBridge(settings.Name).Accessory, accessories...)
	if err != nil {
		log.Fatal(err)
	}

	hc.OnTermination(func() {
		<-t.Stop()
	})
	log.Printf("All functions loaded!\n")
	t.Start()

}

type Bridge struct {
	*accessory.Accessory
}

func NewBridge(name string) *Bridge {
	acc := Bridge{}
	info := accessory.Info{
		Name:         name,
		Manufacturer: "EofNET",
		Model:        settings.Model,
	}
	acc.Accessory = accessory.New(info, accessory.TypeBridge)
	return &acc
}

type Daviklis struct {
	*accessory.Accessory
	Temperature *service.TemperatureSensor
	Humidity    *service.HumiditySensor
}

func NewDaviklis(info accessory.Info) *Daviklis {
	acc := Daviklis{}
	acc.Accessory = accessory.New(info, accessory.TypeThermostat)
	acc.Temperature = service.NewTemperatureSensor()
	acc.Temperature.CurrentTemperature.SetValue(0)
	acc.Humidity = service.NewHumiditySensor()
	acc.Humidity.CurrentRelativeHumidity.SetValue(0)
	acc.AddService(acc.Temperature.Service)
	acc.AddService(acc.Humidity.Service)
	return &acc
}

type TempDaviklis struct {
	*accessory.Accessory
	Temperature *service.TemperatureSensor
}

func NewTempDaviklis(info accessory.Info) *TempDaviklis {
	acc := TempDaviklis{}
	acc.Accessory = accessory.New(info, accessory.TypeThermostat)
	acc.Temperature = service.NewTemperatureSensor()
	acc.Temperature.CurrentTemperature.SetValue(0)
	//        acc.Temperature.Unit = UnitCelcius
	acc.AddService(acc.Temperature.Service)
	return &acc
}

type Fan struct {
	*accessory.Accessory
	Fan *service.Fan
}

// NewFan returns a fan accessory containing one fan service.
func NewFan(info accessory.Info) *Fan {
	acc := Fan{}
	acc.Accessory = accessory.New(info, accessory.TypeFan)
	acc.Fan = service.NewFan()
	acc.AddService(acc.Fan.Service)
	return &acc
}

func Ds18b20Temp(sensor string) (float64, error) {
	data, err := ioutil.ReadFile("/sys/bus/w1/devices/" + sensor + "/w1_slave")
	if err != nil {
		return 0.0, errors.New("failed to read sensor temperature")
	}

	raw := string(data)

	i := strings.LastIndex(raw, "t=")
	if i == -1 {
		return 0.0, errors.New("failed to read sensor temperature")
	}

	c, err := strconv.ParseFloat(raw[i+2:len(raw)-1], 64)
	if err != nil {
		return 0.0, errors.New("failed to read sensor temperature")
	}

	return c / 1000.0, nil
}

// Sensors get all connected sensor IDs as array
func Ds18b20SensorsList() ([]string, error) {
	data, err := ioutil.ReadFile("/sys/bus/w1/devices/w1_bus_master1/w1_master_slaves")
	if err != nil {
		return nil, err
	}

	sensors := strings.Split(string(data), "\n")
	if len(sensors) > 0 {
		sensors = sensors[:len(sensors)-1]
	}

	return sensors, nil
}
