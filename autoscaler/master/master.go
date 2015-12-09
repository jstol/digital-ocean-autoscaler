package master

import (
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jstol/digital-ocean-autoscaler/utils"

	// "github.com/digitalocean/godo"
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/surveyor"
	"github.com/gdamore/mangos/transport/tcp"
)

type master struct {
	url                                           url.URL
	command, configTemplate, configFile           string
	overloadedCpuThreshold, underusedCpuThreshold float64
}

func NewMaster(host, command, configTemplate, configFile string, overloadedCpuThreshold, underusedCpuThreshold float64, minNodes, maxNodes int64) *master {
	bindUrl := url.URL{Scheme: "tcp", Host: host}
	return &master{bindUrl, command, configTemplate, configFile, overloadedCpuThreshold, underusedCpuThreshold}
}

func (m master) shouldAddNode(loadAvgs []float64) bool {
	var avg float64
	for _, loadAvg := range loadAvgs {
		avg += loadAvg
	}
	avg = avg / float64(len(loadAvgs))

	return avg > m.overloadedCpuThreshold
}

func (m master) shouldRemoveNode(loadAvg float64) bool {
	return loadAvg < m.underusedCpuThreshold
}

func (m master) MonitorNodes() {
	var sock mangos.Socket
	var err error
	var msg []byte

	// Try to get new "surveyor" socket
	if sock, err = surveyor.NewSocket(); err != nil {
		utils.Die("Can't get new surveyor socket: %s", err)
	}

	sock.AddTransport(tcp.NewTransport())

	// Begin listening on the URL
	if err = sock.Listen(m.url.String()); err != nil {
		utils.Die("Can't listen on surveyor socket: %s", err.Error())
	}

	// Set "deadline" for the survey
	if err = sock.SetOption(mangos.OptionSurveyTime, time.Second); err != nil {
		utils.Die("SetOption(mangos.OptionSurveyTime): %s", err.Error())
	}
	if err = sock.SetOption(mangos.OptionRecvDeadline, time.Second*2); err != nil {
		utils.Die("SetOption(mangos.OptionRecvDeadline): %s", err.Error())
	}

	// Send out survey requests indefinitely
	for {
		fmt.Println("Sending master request")
		if err = sock.Send([]byte("CPU")); err != nil {
			utils.Die("Failed sending survey: %s", err.Error())
		}

		loadAvgs := []float64{}
		for {
			if msg, err = sock.Recv(); err != nil {
				break
			}
			fmt.Printf("Server: Received \"%s\" survey response\n", string(msg))

			var loadAvg float64
			if loadAvg, err = strconv.ParseFloat(string(msg), 64); err != nil {
				utils.Die("ParseFloat(): %s", err.Error())
			}

			loadAvgs = append(loadAvgs, loadAvg, 64)
		}

		// Make scaling decision
		if m.shouldAddNode(loadAvgs) {
			fmt.Println("Threshold met.")
			var out []byte
			if out, err = exec.Command("sh", "-c", m.command).Output(); err != nil {
				utils.Die("Error executing reload command: %s", err.Error())
			}
			fmt.Printf("Executed command. Output: '%s'\n", strings.TrimSpace(string(out)))
		} else {
			fmt.Println("No extra node needed")
		}

		// Wait
		time.Sleep(time.Second * 3)
	}
}
