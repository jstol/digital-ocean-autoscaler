package main

import (
	"flag"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/jstol/digital-ocean-autoscaler/utils"

	// "github.com/digitalocean/godo"
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/surveyor"
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

	sock.AddTransport(tcp.NewTransport())

	// Begin listening on the URL
	if err = sock.Listen(bind_url.String()); err != nil {
		utils.Die("Can't listen on surveyor socket: %s", err.Error())
	}

	// Set "deadline" for the survey
	if err = sock.SetOption(mangos.OptionSurveyTime, time.Second); err != nil {
		utils.Die("SetOption(mangos.OptionSurveyTime): %s", err.Error())
	}
	if err = sock.SetOption(mangos.OptionRecvDeadline, time.Second*2); err != nil {
		utils.Die("SetOption(mangos.OptionRecvDeadline): %s", err.Error())
	}

	// Send out survey requests
	for {
		fmt.Println("Sending master request")
		if err = sock.Send([]byte("CPU")); err != nil {
			utils.Die("Failed sending survey: %s", err.Error())
		}
		for {
			if msg, err = sock.Recv(); err != nil {
				break
			}
			updateWeights(msg)
			fmt.Printf("Server: Received \"%s\" survey response\n", string(msg))
		}
		time.Sleep(time.Second * 3)
	}
}

func updateWeights(nWeight) {
	var cmd string
	var sokconfig string
	sockconfig = "/etc/haproxy/haproxy.sock"
	s := []string{"set", "weight", "nodes/web1", nWeight}
	cmd = fmt.Printf(strings.Join(s, " "))

	var finalCMD string
	str := []string{"echo", cmd, "|", "socat", "stdio", sockconfig}

	finalCMD = fmt.Printf(strings.Join(str, " "))

	fmt.Println("command is ", cmd)

	out, err := exec.Command(finalCMD).Output()v
	if err != nil {
		fmt.Printf("%s", err)
	}
	fmt.Printf("%s", out)
	wg.Done() // Need to signal to waitgroup that this goroutine is done
}
func main() {
	host := flag.String("host", "0.0.0.0:8000", "the IP address and port")
	flag.Parse()

	fmt.Printf("Starting master at %s\n", *host)
	monitor_nodes(*host)
}
