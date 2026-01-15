package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

type Device struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	MAC        string    `json:"mac"`
	IP         string    `json:"ip"`
	Port       string    `json:"port"`
	IsOnline   bool      `json:"is_online"`
	LastOnline time.Time `json:"last_online"`
}

var (
	devices     []Device
	devicesFile = getDevicesFilePath()
	mutex       sync.Mutex
)

// getDevicesFilePath returns the path to devices.json
// Checks for data directory first (for Docker), otherwise uses current directory
func getDevicesFilePath() string {
	// Check if data directory exists (Docker environment)
	if _, err := os.Stat("data"); err == nil {
		return "data/devices.json"
	}
	return "devices.json"
}

// LoadDevices loads devices from the JSON file.
func LoadDevices() error {
	mutex.Lock()
	defer mutex.Unlock()

	file, err := ioutil.ReadFile(devicesFile)
	if err != nil {
		if os.IsNotExist(err) {
			devices = []Device{}
			return nil
		}
		return err
	}

	return json.Unmarshal(file, &devices)
}

// SaveDevices saves the current list of devices to the JSON file.
// func SaveDevices() error { ... } removed to avoid duplication

// GetAllDevices returns a copy of the devices list safely.
func GetAllDevices() []Device {
	mutex.Lock()
	defer mutex.Unlock()
	result := make([]Device, len(devices))
	copy(result, devices)
	return result
}

func AddDevice(d Device) error {
	mutex.Lock()
	defer mutex.Unlock()
	// Check for duplicates
	for _, dev := range devices {
		if dev.MAC == d.MAC || (dev.IP == d.IP && dev.IP != "") {
			return fmt.Errorf("device with MAC %s or IP %s already exists", d.MAC, d.IP)
		}
	}
	devices = append(devices, d)
	return saveDevicesUnlocked()
}

func RemoveDevice(id string) error {
	mutex.Lock()
	defer mutex.Unlock()
	for i, dev := range devices {
		if dev.ID == id {
			devices = append(devices[:i], devices[i+1:]...)
			return saveDevicesUnlocked()
		}
	}
	return fmt.Errorf("device not found")
}

func GetDevice(id string) (Device, error) {
	mutex.Lock()
	defer mutex.Unlock()
	for _, dev := range devices {
		if dev.ID == id {
			return dev, nil
		}
	}
	return Device{}, fmt.Errorf("device not found")
}

// saveDevicesUnlocked saves the current list of devices to the JSON file.
// Must be called with mutex locked.
func saveDevicesUnlocked() error {
	data, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(devicesFile, data, 0644)
}

// SaveDevices saves the current list of devices to the JSON file.
func SaveDevices() error {
	mutex.Lock()
	defer mutex.Unlock()
	return saveDevicesUnlocked()
}

// Ping performs a ping check on the device's IP.

// Ping performs a ping check on the device's IP.
func Ping(ip string) bool {
	if ip == "" {
		return false
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", "1000", ip)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}

	err := cmd.Run()
	return err == nil
}

// UpdateStatuses updates the online status of all devices.
func UpdateStatuses() {
	mutex.Lock() // Be careful with lock contention, maybe copy list first
	currentDevices := make([]Device, len(devices))
	copy(currentDevices, devices)
	mutex.Unlock()

	// Use waitgroup for parallel pinging
	var wg sync.WaitGroup
	results := make(map[string]bool)
	var resMutex sync.Mutex

	for _, dev := range currentDevices {
		wg.Add(1)
		go func(d Device) {
			defer wg.Done()
			online := Ping(d.IP)
			resMutex.Lock()
			results[d.ID] = online
			resMutex.Unlock()
		}(dev)
	}
	wg.Wait()

	mutex.Lock()
	defer mutex.Unlock()
	for i := range devices {
		if status, ok := results[devices[i].ID]; ok {
			devices[i].IsOnline = status
			if status {
				devices[i].LastOnline = time.Now()
			}
		}
	}
	saveDevicesUnlocked()
}
