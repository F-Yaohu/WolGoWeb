package main

import (
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

type ScannedDevice struct {
	IP  string `json:"ip"`
	MAC string `json:"mac"`
}

// isValidScanIP checks if an IP is valid for device scanning (not multicast, not reserved)
func isValidScanIP(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}

	firstOctet := parts[0]

	// Filter out multicast (224-239)
	if firstOctet == "224" || firstOctet == "239" {
		return false
	}

	// Filter out Class E reserved (240-255)
	if firstOctet >= "240" {
		return false
	}

	// Filter out loopback (127.x.x.x)
	if firstOctet == "127" {
		return false
	}

	// Filter out link-local (169.254.x.x)
	if firstOctet == "169" && parts[1] == "254" {
		return false
	}

	return true
}

// isValidMAC checks if a MAC address is valid for device scanning
func isValidMAC(mac string) bool {
	mac = strings.ToLower(mac)

	// Filter broadcast
	if strings.HasPrefix(mac, "ff:ff:ff") {
		return false
	}

	// Filter multicast MACs (01:00:5e:xx:xx:xx for IPv4 multicast)
	if strings.HasPrefix(mac, "01:00:5e") {
		return false
	}

	// Filter IPv6 multicast MACs (33:33:xx:xx:xx:xx)
	if strings.HasPrefix(mac, "33:33") {
		return false
	}

	// Filter incomplete MACs (00:00:00:00:00:00)
	if mac == "00:00:00:00:00:00" {
		return false
	}

	return true
}

// ScanLocalNetwork attempts to find devices by pinging the subnet and reading the ARP table.
func ScanLocalNetwork() ([]ScannedDevice, error) {
	// 1. Determine local subnet
	ip, ipNet, err := getLocalIPAndNetwork()
	if err != nil {
		return nil, err
	}

	// 2. Ping all IPs in subnet to populate ARP table
	wg := sync.WaitGroup{}

	// Calculate network size
	ones, bits := ipNet.Mask.Size()
	maskSize := bits - ones
	hostCount := (1 << maskSize) - 2 // Exclude network and broadcast

	// Limit scanning to reasonable size (e.g., max 512 hosts)
	if hostCount > 512 {
		hostCount = 512
	}

	// Semaphore to limit concurrency (increased for faster scanning)
	sem := make(chan struct{}, 100)

	// Generate IPs to scan
	baseIP := ip.Mask(ipNet.Mask)
	for i := 1; i <= hostCount; i++ {
		// Calculate target IP
		targetIP := make(net.IP, len(baseIP))
		copy(targetIP, baseIP)

		// Add offset (works for /24, /23, /22, etc.)
		offset := i
		for j := len(targetIP) - 1; j >= 0 && offset > 0; j-- {
			sum := int(targetIP[j]) + offset
			targetIP[j] = byte(sum % 256)
			offset = sum / 256
		}

		if targetIP.Equal(ip) {
			continue // skip self
		}

		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			sem <- struct{}{}
			fastPing(target)
			<-sem
		}(targetIP.String())
	}
	wg.Wait()

	// Wait a bit for ARP cache to stabilize
	time.Sleep(500 * time.Millisecond)

	// 3. Read ARP table
	return getArpTable()
}

func fastPing(ip string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows ping: -n 1 count, -w 500 timeout (ms) - increased timeout
		cmd = exec.Command("ping", "-n", "1", "-w", "500", ip)
	} else {
		// Linux/Mac ping: -c 1 count, -W 1 timeout (sec)
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}

	// Try ping, if failed, try once more
	if err := cmd.Run(); err != nil {
		// Retry once for unreliable networks
		if runtime.GOOS == "windows" {
			cmd = exec.Command("ping", "-n", "1", "-w", "500", ip)
		} else {
			cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
		}
		cmd.Run()
	}
}

func getLocalIPAndNetwork() (net.IP, *net.IPNet, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, nil, err
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP, ipnet, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("no valid local IP found")
}

func getArpTable() ([]ScannedDevice, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("arp", "-a")
	} else {
		cmd = exec.Command("arp", "-a")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseArpOutput(string(output)), nil
}

func parseArpOutput(output string) []ScannedDevice {
	var devices []ScannedDevice
	lines := strings.Split(output, "\n")

	// Regular expressions for IP and MAC
	// Windows: "  192.168.1.1          11-22-33-44-55-66     dynamic"
	// Linux/Mac: "? (192.168.1.1) at 11:22:33:44:55:66 [ether] on eth0"

	// Simple approach: Look for lines containing both an IP and a MAC
	reIP := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
	// MAC regex usually matches AA:BB:CC... or AA-BB-CC...
	reMAC := regexp.MustCompile(`([0-9a-fA-F]{2}[:-][0-9a-fA-F]{2}[:-][0-9a-fA-F]{2}[:-][0-9a-fA-F]{2}[:-][0-9a-fA-F]{2}[:-][0-9a-fA-F]{2})`)

	seen := make(map[string]bool)

	for _, line := range lines {
		ipMatch := reIP.FindString(line)
		macMatch := reMAC.FindString(line)

		if ipMatch != "" && macMatch != "" {
			// Normalize MAC to colon-separated and uppercase
			normalizedMAC := strings.ToUpper(strings.ReplaceAll(macMatch, "-", ":"))

			// Filter invalid IPs and MACs
			if !isValidScanIP(ipMatch) {
				continue
			}

			if !isValidMAC(normalizedMAC) {
				continue
			}

			if !seen[normalizedMAC] {
				devices = append(devices, ScannedDevice{
					IP:  ipMatch,
					MAC: normalizedMAC,
				})
				seen[normalizedMAC] = true
			}
		}
	}
	return devices
}
