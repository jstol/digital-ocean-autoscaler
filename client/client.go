package main

import (
	"flag"
	"fmt"
	"net"
	"net/url"

	"github.com/jstol/digital-ocean-autoscaler/utils"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/respondent"
	"github.com/gdamore/mangos/transport/tcp"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/load"
)

func getPrivateIP() string {
	i, err := net.InterfaceByName("eth1")
	if err != nil {
		utils.Die("Error getting interface eth1: %s\n", err.Error())
	}

	addrs, err := i.Addrs()
	if err != nil {
		utils.Die("Error getting interface eth1 addresses: %s\n", err.Error())
	}

	var ip string
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
				break
			}
		}
	}
	return ip
}

func startNode(masterHost, name string) {
	var sock mangos.Socket
	var err error
	var msg []byte
	masterUrl := url.URL{Scheme: "tcp", Host: masterHost}
	ip := getPrivateIP()

	// Try to get new "respondent" socket
	if sock, err = respondent.NewSocket(); err != nil {
		utils.Die("Can't get new respondent socket: %s", err.Error())
	}
	defer sock.Close()

	sock.AddTransport(tcp.NewTransport())

	// Connect to master
	if err = sock.Dial(masterUrl.String()); err != nil {
		utils.Die("Can't dial on respondent socket: %s", err.Error())
	}

	// Wait for a survey request and send responses
	for {
		if msg, err = sock.Recv(); err != nil {
			utils.Die("Cannot recv: %s", err.Error())
		}
		fmt.Printf("Client(%s): Received \"%s\" survey request\n", name, string(msg))

		var loadAvg *load.LoadAvgStat
		if loadAvg, err = load.LoadAvg(); err != nil {
			utils.Die("Cannot get load average: %s", err.Error())
		}

		var cpuInfo []cpu.CPUInfoStat
		if cpuInfo, err = cpu.CPUInfo(); err != nil {
			utils.Die("Cannot get CPU info: %s", err.Error())
		}
		fmt.Printf("CPU INFO len: %d\n", len(cpuInfo))

		// Get the normalized CPU load
		avg := loadAvg.Load1
		cores := int32(0)
		for _, info := range cpuInfo {
			fmt.Printf("Inner Cores: %d\n", info.Cores)
			cores += info.Cores
		}
		fmt.Printf("Load avg: %f\n", avg)
		fmt.Printf("Cores: %d\n", cores)
		avg = avg / float64(cores)

		fmt.Printf("Client(%s): Sending survey response\n", name)
		if err = sock.Send([]byte(fmt.Sprintf("%s,%f", ip, avg))); err != nil {
			utils.Die("Cannot send: %s", err.Error())
		}
	}
}

func main() {
	host := flag.String("host", "", "the IP address and port")
	clientId := flag.Int64("id", 1, "the id of the node")
	flag.Parse()

	if *host == "" {
		utils.Die("No host address provided")
	}

	fmt.Printf("Starting client. Connecting to master at %s\n", *host)
	startNode(*host, fmt.Sprintf("%d", *clientId))
}
