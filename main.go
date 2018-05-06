package main

import (
	"os"
	// stdlib
	"log"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	// Private IP ranges.
	v4PrivateRanges = []ipCIDR{
		ipCIDR{cidr: "10.0.0.0/8"},
		ipCIDR{cidr: "172.16.0.0/12"},
		ipCIDR{cidr: "192.168.0.0/16"},
	}
	v6PrivateRanges = []ipCIDR{
		ipCIDR{cidr: "fd00::/8"},
	}
	// Terminals we can use for showing logs and errors.
	terminals = []string{
		"gnome-terminal",
		"konsole",
		"urxvt",
		"xfce4-terminal",
		"xterm",
	}
)

// Structure that holds IP CIRD.
type ipCIDR struct {
	cidr string
}

func main() {
	addresses, errors := getIps()
	if len(errors) > 0 {
		log.Print(errors)
	}
	showIps(addresses)
}

// IPv4 address checks.
func checksForIPv4(address net.IP) bool {
	log.Printf("Checking IPv4 address '%s' for usableness...", address.String())

	// Next checks are performed only for logging purposes!
	// We do not need multicast addresses (if suddenly).
	if address.IsMulticast() || address.IsLinkLocalMulticast() || address.IsInterfaceLocalMulticast() {
		log.Printf("Can't use Multicast IPv4 address! Removing '%s'  from list of usable addresses", address.String())
		return false
	}
	// If loopback address suddenly appears here - we should not use it.
	if address.IsLoopback() {
		log.Printf("Can't use loopback address! Removing '%s' from list of usable addresses", address.String())
		return false
	}
	// We should not use link-local addresses.
	if address.IsLinkLocalUnicast() {
		log.Printf("Can't use link-local addresses! Removing '%s' from list of usable addresses", address.String())
		return false
	}

	// Global Unicast addresses contains private ranges, so we should check
	// ranges here.
	if address.IsGlobalUnicast() {
		local_address := false
		// Check if address within private addresses ranges.
		for _, network := range v4PrivateRanges {
			_, cidr, _ := net.ParseCIDR(network.cidr)
			if cidr.Contains(address) {
				local_address = true
			}
		}

		if local_address {
			log.Printf("Address '%s' looks good.", address.String())
			return true
		}
	}

	// All other things - FALSE.
	return false
}

// IPv6 address checks.
func checksForIPv6(address net.IP) bool {
	log.Printf("Checking IPv6 address '%s' for usableness...", address.String())

	// Next checks are performed only for logging purposes!
	// We do not need multicast addresses (if suddenly).
	if address.IsMulticast() || address.IsLinkLocalMulticast() || address.IsInterfaceLocalMulticast() {
		log.Printf("Can't use Multicast IPv4 address! Removing '%s' from list of usable addresses", address.String())
		return false
	}
	// If loopback address suddenly appears here - we should not use it.
	if address.IsLoopback() {
		log.Printf("Can't use loopback address! Removing '%s' from list of usable addresses", address.String())
		return false
	}
	// We should not use link-local addresses.
	if address.IsLinkLocalUnicast() {
		log.Printf("Can't use link-local addresses! Removing '%s' from list of usable addresses", address.String())
		return false
	}

	// Global Unicast addresses contains private ranges, so we should check
	// ranges here.
	if address.IsGlobalUnicast() {
		local_address := false
		// Check if address within private addresses ranges.
		for _, network := range v6PrivateRanges {
			_, cidr, _ := net.ParseCIDR(network.cidr)
			if cidr.Contains(address) {
				local_address = true
			}
		}

		if local_address {
			log.Printf("Address '%s' looks good.", address.String())
			return true
		}
	}

	// All other things - FALSE.
	return false
}

func getIps() ([]string, []error) {
	log.Print("Getting all available IP addresses...")

	var addresses []string
	var errors []error

	interfacesRaw, err := net.Interfaces()
	if err != nil {
		errors = append(errors, err)
	}

	// Check interfaces for usabillness.
	var usableIfaces []string
	for i := range interfacesRaw {
		// Ignore unneeded addresses.
		if interfacesRaw[i].Flags&net.FlagLoopback != 0 {
			continue
		}
		// We should also ignore Point-to-Point addresses, because they
		// should not be used on production server/VM.
		if interfacesRaw[i].Flags&net.FlagPointToPoint != 0 {
			continue
		}
		// Also we should skip interfaces that didn't have "UP" state.
		if interfacesRaw[i].Flags&net.FlagUp == 0 {
			continue
		}
		// Bridges? IGNORE!
		if strings.Contains(interfacesRaw[i].Name, "br") {
			continue
		}

		usableIfaces = append(usableIfaces, interfacesRaw[i].Name)
	}

	log.Print("Found interfaces:")
	log.Print(usableIfaces)

	// Get interfaces IP addresses.
	for i := range usableIfaces {
		// Get interface.
		iface, err1 := net.InterfaceByName(usableIfaces[i])
		if err1 != nil {
			errors = append(errors, err1)
			continue
		}
		// Get addresses
		addressesRaw, err2 := iface.Addrs()
		if err2 != nil {
			errors = append(errors, err2)
			continue
		}

		// Check for addresses usabillness.
		for ii := range addressesRaw {
			addressRaw, _, err3 := net.ParseCIDR(addressesRaw[ii].String())
			if err3 != nil {
				errors = append(errors, err3)
				continue
			}
			var usable = false
			if addressRaw.To4() != nil {
				usable = checksForIPv4(addressRaw)
			} else {
				usable = checksForIPv6(addressRaw)
			}

			if usable {
				addresses = append(addresses, addressRaw.String())
			}
		}
	}

	return addresses, errors
}

func showIps(ips []string) {
	ipsAsString := strings.Join(ips, ",")
	log.Print("IPs string: " + ipsAsString)

	// Find zenity.
	var paths []string
	path, pathFound := os.LookupEnv("PATH")
	if !pathFound {
		log.Print("PATH variable isn't defined or empty. Looking in default locations.")
		paths = []string{"/bin", "/usr/bin", "/usr/local/bin"}
	} else {
		paths = strings.Split(path, ":")
	}

	log.Print("Looking for apps in these paths:")
	log.Print(paths)

	var zenity string
	var terminal string
	for i := range paths {
		if zenity == "" {
			zenityTempPath := filepath.Join(paths[i], "zenity")
			if _, err := os.Stat(zenityTempPath); err == nil {
				zenity = zenityTempPath
			}
		}

		if terminal == "" {
			for ii := range terminals {
				terminalTempPath := filepath.Join(paths[i], terminals[ii])
				if _, err := os.Stat(terminalTempPath); err == nil {
					terminal = terminalTempPath
				}
			}
		}

		if terminal != "" && zenity != "" {
			break
		}
	}

	if zenity == "" {
		log.Fatal("Failed to find Zenity binary!")
	}

	log.Print("Will use:")
	log.Printf("\tTerminal at '%s'", terminal)
	log.Printf("\tZenity at '%s'", zenity)

	ipsText := "Your IP addresses: " + ipsAsString

	zenityCmd := exec.Command(zenity, "--info", "--title=IPShow", "--text="+ipsText, "--no-wrap")
	err := zenityCmd.Run()
	if err != nil {
		log.Print(err.Error())
	}
}
