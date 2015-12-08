package main

import (
	"flag"
	"fmt"
	"net/url"

	"github.com/jstol/digital-ocean-autoscaler/utils"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/respondent"
	"github.com/gdamore/mangos/transport/tcp"
	"github.com/shirou/gopsutil/load"
)

func start_node(master_host string, name string) {
	var sock mangos.Socket
	var err error
	var msg []byte
	master_url := url.URL{Scheme: "tcp", Host: master_host}

	// Try to get new "respondent" socket
	if sock, err = respondent.NewSocket(); err != nil {
		utils.Die("Can't get new respondent socket: %s", err.Error())
	}

	sock.AddTransport(tcp.NewTransport())

	// Connect to master
	if err = sock.Dial(master_url.String()); err != nil {
		utils.Die("Can't dial on respondent socket: %s", err.Error())
	}

	// Wait for a survey request and send responses
	for {
		if msg, err = sock.Recv(); err != nil {
			utils.Die("Cannot recv: %s", err.Error())
		}
		fmt.Printf("Client(%s): Received \"%s\" survey request\n", name, string(msg))

		var load_avg *load.LoadAvgStat
		if load_avg, err = load.LoadAvg(); err != nil {
			utils.Die("Cannot get load average: %s", err.Error())
		}

		avg := load_avg.Load1
		fmt.Printf("Client(%s): Sending survey response\n", name)
		if err = sock.Send([]byte(fmt.Sprintf("%2f", avg))); err != nil {
			utils.Die("Cannot send: %s", err.Error())
		}
	}
}

func main() {
	host := flag.String("host", "0.0.0.0:8000", "the IP address and port")
	flag.Parse()

	fmt.Printf("Starting client. Connecting to master at %s\n", *host)
	start_node(*host, "1")
}
