package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

//InterfaceStatus - used to store info about interface, it's bridges, bonds, etc
type InterfaceStatus struct {
	name       string
	operstate  string
	carrier    string
	mtu        string
	duplex     string
	speed      string
	txQueueLen string
	isbridge   bool
	briflist   []BridgeInterfaces
	isbond     bool
	bondifnum  int
	bondoperst bool
	bondiflist []BondInterfaces
}

//BridgeInterfaces - stores name and state of bridge interface
type BridgeInterfaces struct {
	name  string
	state string
}

//BondInterfaces - stores name and state of bonded interface
type BondInterfaces struct {
	name  string
	state string
}

//InterfacesInfo slice of interfaces
var InterfacesInfo []InterfaceStatus

//Syspath - define variable with path to sysfs network section
var Syspath = "/sys/class/net/"

//list of flags
var ignoreif = flag.String("ignoreif", "lo", "list of interfaces that will be ignored")
var debug = flag.Bool("debug", false, "enable debug mode")
var ubuntu = flag.Bool("ubuntu", false, "get's list of interfaces from ubuntu's /etc/network/interfaces")

//get list of interfaces and tore it at InterfacesInfo var
func getIfs() []string {
	interfaces := []string{}
	files, _ := ioutil.ReadDir(Syspath)
	if *ubuntu {
		// if ubuntu flag true search for line at interfaces config that starts with auto
		ifConfFile, err := ioutil.ReadFile("/etc/network/interfaces")
		check(err)
		for _, v := range strings.Split(string(ifConfFile), "\n") {

			if strings.HasPrefix(v, "auto ") {
				v = strings.Replace(v, "lo ", "", -1)
				for _, ifc := range strings.Split(string(v), " ") {
					if ifc != "auto" {
						interfaces = append(interfaces, Syspath+ifc)
					}
				}

			}
		}
	} else {
		for _, f := range files {
			fullpath := Syspath + f.Name()
			fi, err := os.Lstat(fullpath)
			check(err)
			if (fi.Mode()&os.ModeSymlink == os.ModeSymlink) && (f.Name() != "lo") {
				interfaces = append(interfaces, fullpath)
			}
		}
	}
	return interfaces
}

//get needed info about specified interface
func getIfInfo(intf string) {
	isBridge := false
	isBond := false
	bondNumIf := 0
	bondFullOperational := true
	bridgeIfList := []BridgeInterfaces{}
	bondIfList := []BondInterfaces{}
	ifstatus := "/operstate"
	if _, err := os.Stat(intf + "/brif/"); err == nil {
		isBridge = true
		files, _ := ioutil.ReadDir(intf + "/brif/")
		for _, f := range files {
			state, err := ioutil.ReadFile(intf + "/brif/" + f.Name() + "/state")
			check(err)
			bridgeIfList = append(bridgeIfList, BridgeInterfaces{
				strings.TrimSpace(f.Name()),
				strings.TrimSpace(string(state))})
		}
	}
	operstate, err := ioutil.ReadFile(intf + ifstatus)
	carrier, err := ioutil.ReadFile(intf + "/carrier")
	mtu, err := ioutil.ReadFile(intf + "/mtu")
	duplex, err := ioutil.ReadFile(intf + "/duplex")
	speed, err := ioutil.ReadFile(intf + "/speed")
	txQueueLen, err := ioutil.ReadFile(intf + "/tx_queue_len")

	check(err)

	if _, err := os.Stat(intf + "/bonding/"); err == nil && strings.TrimSpace(string(operstate)) == "up" {
		interfacenumber := 0
		bondNumPorts, err := ioutil.ReadFile(intf + "/bonding/ad_num_ports")
		check(err)

		slaves, err := ioutil.ReadFile(intf + "/bonding/slaves")
		if len(slaves) != 0 {
			for _, i := range strings.Split(strings.TrimSpace(string(slaves)), " ") {
				interfacenumber++
				bondifoperstate, err := ioutil.ReadFile(Syspath + i + "/operstate")
				check(err)
				bondIfList = append(bondIfList, BondInterfaces{
					i,
					strings.TrimSpace(string(bondifoperstate))})
			}
		} else {
			bondFullOperational = false
		}
		bondNumIf, err = strconv.Atoi(strings.TrimSpace(string(bondNumPorts)))
		if err != nil {
			bondNumIf = 0
		}
		if bondNumIf != interfacenumber {
			bondFullOperational = false
		}
		isBond = true
		ifstatus = "/bonding/mii_status"
	}

	//add to InterfacesInfo slice info about interface, it's parameters, and data if it's bridge or bond and which interfaces are there
	InterfacesInfo = append(InterfacesInfo, InterfaceStatus{
		strings.TrimSpace(strings.Split(intf, "/")[4]),
		strings.TrimSpace(string(operstate)),
		strings.TrimSpace(string(carrier)),
		strings.TrimSpace(string(mtu)),
		strings.TrimSpace(string(duplex)),
		strings.TrimSpace(string(speed)),
		strings.TrimSpace(string(txQueueLen)),
		isBridge,
		bridgeIfList,
		isBond,
		bondNumIf,
		bondFullOperational,
		bondIfList})

}

//show interface info (useful for debug)
func showIfStatus() {
	for _, item := range InterfacesInfo {
		fmt.Println("Interface name: " + item.name)
		fmt.Println("Interface is bridge?: " + strconv.FormatBool(item.isbridge))
		fmt.Println("Interface is bonded interface?: " + strconv.FormatBool(item.isbond))
		fmt.Println("Operational state: " + item.operstate)
		fmt.Println("Carrier: " + item.carrier)
		fmt.Println("MTU: " + item.mtu)
		fmt.Println("Duplex: " + item.duplex)
		fmt.Println("Speed: " + item.speed)
		fmt.Println("Tx queue length: " + item.txQueueLen)
		for _, bri := range item.briflist {
			fmt.Println(bri.name)
		}
		fmt.Println("-----------------------------------------")
	}

}

//process collected info
func sensuIfStatus() (string, int) {
	var ProblemInterfaces = "Problem interfaces: "
	var ExitCode int
	var ifCarrier = "_carrier:up "
	IgnoredIf := strings.Split(*ignoreif, ",")
	for _, item := range InterfacesInfo {
		ifCarrier = " "
		if stringInSlice(item.name, IgnoredIf) {
			fmt.Println("Interface: " + item.name + " ignored")
		} else {
			if item.carrier != "1" {
				ifCarrier = "_carrier:down "
			}
			switch {
			case checkIfBridge(item.name) == true:
				for _, brif := range item.briflist {
					if (brif.state != "3") && (ExitCode == 0) {
						ProblemInterfaces = ProblemInterfaces + " " + brif.name + ":" + "bridge.interface.non-operational" + ifCarrier
						ExitCode = 2
					}
				}
			case item.isbond == true:
				if !item.bondoperst {
					for _, boif := range item.bondiflist {
						if boif.state != "up" {
							ProblemInterfaces = ProblemInterfaces + " " + boif.name + "@" + item.name + ":bond.interface" + ifCarrier
							ExitCode = 1
						}
					}

				} else {
					if item.operstate != "up" {
						ProblemInterfaces = ProblemInterfaces + item.name + ":bond.interface.problem" + ifCarrier
						ExitCode = 2
					}
				}
			default:
				switch {
				case item.operstate == "unknown":
					ProblemInterfaces = ProblemInterfaces + " " + item.name + ":" + "unknown "
					if ExitCode == 0 {
						ExitCode = 1
					}
				case item.operstate == "down":
					ProblemInterfaces = ProblemInterfaces + " " + item.name + ":" + "down" + ifCarrier
					ExitCode = 2
				}
			}
		}
	}

	return ProblemInterfaces, ExitCode

}

//check if interface is at bridge
func checkIfBridge(netinerface string) bool {
	IfInBridge := false
	for _, item := range InterfacesInfo {
		for _, bri := range item.briflist {
			if (bri.name == netinerface) && (IfInBridge == false) {
				IfInBridge = true
			}
		}
	}
	return IfInBridge

}

//process info about interfaces
func processIfInfo() {
	intfc := getIfs()
	for _, f := range intfc {
		//exclude aliases
		if !(strings.Contains(f, ":")) {
			getIfInfo(f)
		}
	}
}

//function for checking if interface is at list of ignored interfaces
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

//error checking function
func check(e error) {
	if e != nil {
		panic(e)
	}
}

//main function
func main() {
	flag.Parse()
	processIfInfo()
	pr, ec := sensuIfStatus()
	if *debug == true {
		showIfStatus()
	}
	if ec != 0 {
		fmt.Println(pr)
	} else {
		fmt.Println("All interfaces are operational")
	}
	os.Exit(ec)
}
