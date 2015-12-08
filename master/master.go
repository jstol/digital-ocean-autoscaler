package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/jstol/digital-ocean-autoscaler/utils"

	// "github.com/digitalocean/godo"
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/surveyor"
	"github.com/gdamore/mangos/transport/ipc"
	"github.com/gdamore/mangos/transport/tcp"
)

func monitor_nodes(host string) {
	var sock mangos.Socket
	var err error
	var msg []byte
	bind_url := url.URL{Scheme: "tcp", Host: host}

	// Try to get new "surveyor" socket
	if sock, err = surveyor.NewSocket(); err != nil {
		utils.Die("Can't get new surveyor socket: %s", err)
	}

	sock.AddTransport(ipc.NewTransport())
	sock.AddTransport(tcp.NewTransport())

	// Begin listening on the URL
	if err = sock.Listen(bind_url.String()); err != nil {
		utils.Die("Can't listen on surveyor socket: %s", err.Error())
	}

	// Set "deadline" for the survey
	err = sock.SetOption(mangos.OptionSurveyTime, time.Second*2)
	if err != nil {
		utils.Die("SetOption(): %s", err.Error())
	}

	// Send out survey requiests
	for {
		fmt.Println("Sending master request")
		if err = sock.Send([]byte("CPU")); err != nil {
			utils.Die("Failed sending survey: %s", err.Error())
		}
		for {
			if msg, err = sock.Recv(); err != nil {
				break
			}
			fmt.Printf("Server: received \"%s\" survey response\n", string(msg))
		}
	}
}

func main() {
	fmt.Printf("Starting master at localhost:8080\n")
	monitor_nodes("localhost:8080")
}
