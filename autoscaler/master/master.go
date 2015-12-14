package master

import (
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jstol/digital-ocean-autoscaler/utils"

	"github.com/digitalocean/godo"
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/surveyor"
	"github.com/gdamore/mangos/transport/tcp"
	"golang.org/x/oauth2"
)

// TokenSource type for Digital Ocean client
type TokenSource struct {
	AccessToken string
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

// Master type
type master struct {
	url                                           url.URL
	command, configTemplate, configFile           string
	overloadedCpuThreshold, underusedCpuThreshold float64
	minNodes, maxNodes                            int64
	token                                         *TokenSource
	doClient                                      *godo.Client
}

func NewMaster(host, command, configTemplate, configFile, digitalOceanToken string, overloadedCpuThreshold, underusedCpuThreshold float64, minNodes, maxNodes int64) *master {
	bindUrl := url.URL{Scheme: "tcp", Host: host}

	// Set up the Digital Ocean client
	tokenSource := &TokenSource{
		AccessToken: digitalOceanToken,
	}
	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	client := godo.NewClient(oauthClient)

	return &master{
		url:                    bindUrl,
		command:                command,
		configTemplate:         configTemplate,
		configFile:             configFile,
		overloadedCpuThreshold: overloadedCpuThreshold,
		underusedCpuThreshold:  underusedCpuThreshold,
		minNodes:               minNodes,
		maxNodes:               maxNodes,
		token:                  tokenSource,
		doClient:               client,
	}
}

func (m master) querySlaves(c chan<- []float64) {
	var err error
	var msg []byte
	var sock mangos.Socket

	// Try to get new "surveyor" socket
	if sock, err = surveyor.NewSocket(); err != nil {
		utils.Die("Can't get new surveyor socket: %s", err)
	}

	sock.AddTransport(tcp.NewTransport())

	// Begin listening on the URL
	if err = sock.Listen(m.url.String()); err != nil {
		utils.Die("Can't listen on surveyor socket: %s", err.Error())
	}

	// Set "deadline" for the survey and a timeout for receiving responses
	if err = sock.SetOption(mangos.OptionSurveyTime, time.Second); err != nil {
		utils.Die("SetOption(mangos.OptionSurveyTime): %s", err.Error())
	}
	if err = sock.SetOption(mangos.OptionRecvDeadline, time.Second*2); err != nil {
		utils.Die("SetOption(mangos.OptionRecvDeadline): %s", err.Error())
	}

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

			loadAvgs = append(loadAvgs, loadAvg)
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

		// Send the load averages
		c <- loadAvgs

		// Wait
		time.Sleep(time.Second * 3)
	}
}

func (m master) shouldAddNode(loadAvgs []float64) bool {
	var avg float64
	for _, loadAvg := range loadAvgs {
		avg += loadAvg
	}
	avg = avg / float64(len(loadAvgs))

	return avg > m.overloadedCpuThreshold
}

func (m master) addNode() error {
	// createRequest := &godo.DropletCreateRequest{
	// 	Name:   dropletName,
	// 	Region: "tor1",
	// 	Size:   "512mb",
	// 	Image: godo.DropletCreateImage{
	// 		Slug: "ubuntu-14-04-x64",
	// 	},
	// }

	// // TODO do something with the droplet
	// newDroplet, _, err := client.Droplets.Create(createRequest)

	// if err != nil {
	// 	utils.Die("Couldn't create droplet: %s\n", err.Error())
	// }
	return nil
}

func (m master) shouldRemoveNode(loadAvg float64) bool {
	return loadAvg < m.underusedCpuThreshold
}

func (m master) MonitorNodes() {
	// var err error

	// Send out survey requests indefinitely
	loadAvgsChannel := make(chan []float64)
	// dropletPollChannel := make

	// Start worker threads
	go m.querySlaves(loadAvgsChannel)

	for {
		select {
		case loadAvgs := <-loadAvgsChannel:
			fmt.Printf("GOT LOADAVGS %d\n", len(loadAvgs))
			for _, element := range loadAvgs {
				fmt.Println(element)
			}
		}
	}
}
