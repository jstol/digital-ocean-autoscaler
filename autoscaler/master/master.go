package master

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/jstol/digital-ocean-autoscaler/utils"

	"github.com/digitalocean/godo"
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/surveyor"
	"github.com/gdamore/mangos/transport/tcp"
	"github.com/quipo/statsd"
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
type Master struct {
	url                                                           url.URL
	workerConfig                                                  *WorkerConfig
	droplets                                                      []godo.Droplet
	command, balanceConfigTemplate, balanceConfigFile, imageID    string
	currentLoadAvg, overloadedCpuThreshold, underusedCpuThreshold float64
	minWorkers, maxWorkers, workerCount                           int64
	waitingOnWorkerChange, coolingDown                            bool
	token                                                         *TokenSource
	doClient                                                      *godo.Client
	pollInterval, cooldownInterval, surveyDeadline, queryInterval time.Duration
	statsdClientBuffer                                            *statsd.StatsdBuffer
}

func NewMaster(host string, workerConfig *WorkerConfig, command, balanceConfigTemplate, balanceConfigFile, digitalOceanToken, digitalOceanImageID string,
	overloadedCpuThreshold, underusedCpuThreshold float64, minWorkers, maxWorkers int64, pollInterval, cooldownInterval, surveyDeadline, queryInterval time.Duration) *Master {

	var err error
	bindUrl := url.URL{Scheme: "tcp", Host: host}

	// Set up the Digital Ocean client
	tokenSource := &TokenSource{
		AccessToken: digitalOceanToken,
	}
	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	client := godo.NewClient(oauthClient)

	// Create a set containing the configured worker nodes
	workerSet := make(map[string]interface{})
	for _, name := range workerConfig.DropletNames {
		workerSet[name] = nil
	}

	// Get a list of all of the droplets and filter out any irrelevant ones
	var workerDroplets, allDroplets []godo.Droplet
	if allDroplets, _, err = client.Droplets.List(nil); err != nil {
		utils.Die("Error getting the list of droplets: %s", err)
	}

	for _, droplet := range allDroplets {
		if _, contains := workerSet[droplet.Name]; contains {
			workerDroplets = append(workerDroplets, droplet)
		}
	}

	return &Master{
		url:                    bindUrl,
		workerConfig:           workerConfig,
		droplets:               workerDroplets,
		command:                command,
		balanceConfigTemplate:  balanceConfigTemplate,
		balanceConfigFile:      balanceConfigFile,
		overloadedCpuThreshold: overloadedCpuThreshold,
		underusedCpuThreshold:  underusedCpuThreshold,
		minWorkers:             minWorkers,
		maxWorkers:             maxWorkers,
		token:                  tokenSource,
		imageID:                digitalOceanImageID,
		doClient:               client,
		pollInterval:           pollInterval,
		cooldownInterval:       cooldownInterval,
		surveyDeadline:         surveyDeadline,
		queryInterval:          queryInterval,
	}
}

func NewMasterWithStatsd(host string, workerConfig *WorkerConfig, command, balanceConfigTemplate, balanceConfigFile, digitalOceanToken, digitalOceanImageID string,
	overloadedCpuThreshold, underusedCpuThreshold float64, minWorkers, maxWorkers int64, pollInterval, cooldownInterval, surveyDeadline, queryInterval time.Duration, statsdAddr, statsdPrefix string, statsdInterval time.Duration) *Master {

	master := NewMaster(
		host, workerConfig, command,
		balanceConfigTemplate, balanceConfigFile,
		digitalOceanToken, digitalOceanImageID,
		overloadedCpuThreshold, underusedCpuThreshold,
		minWorkers, maxWorkers,
		pollInterval, cooldownInterval, surveyDeadline, queryInterval,
	)
	statsdClient := statsd.NewStatsdClient(statsdAddr, statsdPrefix)
	statsdClient.CreateSocket()
	master.statsdClientBuffer = statsd.NewStatsdBuffer(statsdInterval, statsdClient)

	return master
}

func (m *Master) cooldown() {
	m.coolingDown = true
	time.Sleep(m.cooldownInterval)
	m.coolingDown = false
}

func (m *Master) queryWorkers(c chan<- float64) {
	var err error
	var msg []byte
	var sock mangos.Socket

	// Try to get new "surveyor" socket
	if sock, err = surveyor.NewSocket(); err != nil {
		utils.Die("Can't get new surveyor socket: %s", err)
	}
	defer sock.Close()

	sock.AddTransport(tcp.NewTransport())

	// Begin listening on the URL
	if err = sock.Listen(m.url.String()); err != nil {
		utils.Die("Can't listen on surveyor socket: %s", err.Error())
	}

	// Set "deadline" for the survey and a timeout for receiving responses
	if err = sock.SetOption(mangos.OptionSurveyTime, m.surveyDeadline); err != nil {
		utils.Die("SetOption(mangos.OptionSurveyTime): %s", err.Error())
	}
	if err = sock.SetOption(mangos.OptionRecvDeadline, m.surveyDeadline+(1*time.Second)); err != nil {
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

		// Compute the average loadAvg
		var loadAvg float64
		for _, avg := range loadAvgs {
			loadAvg += avg
		}
		loadAvg /= float64(len(loadAvgs))

		// Send the load averages
		if !math.IsNaN(loadAvg) {
			c <- loadAvg
		}

		// Wait
		time.Sleep(m.queryInterval)
	}
}

func (m *Master) shouldAddWorker(loadAvg float64) bool {
	return !m.waitingOnWorkerChange && !m.coolingDown && loadAvg > m.overloadedCpuThreshold && int64(len(m.droplets)) < m.maxWorkers
}

func (m *Master) addWorker(c chan<- *godo.Droplet) {
	var (
		droplet *godo.Droplet
		err     error
	)

	// TODO find a better way to dynamically name the workers, create using a snapshot
	name := fmt.Sprintf("%s%d", m.workerConfig.NamePrefix, len(m.droplets)+1)
	createRequest := &godo.DropletCreateRequest{
		Name:              name,
		Region:            "tor1",
		Size:              "512mb",
		PrivateNetworking: true,
		Image: godo.DropletCreateImage{
			Slug: m.imageID,
		},
	}

	if droplet, _, err = m.doClient.Droplets.Create(createRequest); err != nil {
		utils.Die("Couldn't create droplet: %s\n", err.Error())
	}

	for {
		time.Sleep(m.pollInterval)
		if droplet, _, _ = m.doClient.Droplets.Get(droplet.ID); droplet.Status == "active" {
			break
		}

		fmt.Printf("Polling. Status: %s\n", droplet.Status)
	}

	fmt.Println("Droplet creation complete")

	c <- droplet
}

func (m *Master) shouldRemoveWorker(loadAvg float64) bool {
	return !m.waitingOnWorkerChange && !m.coolingDown && loadAvg < m.underusedCpuThreshold && int64(len(m.droplets)) > m.minWorkers
}

func (m *Master) removeWorker(c chan<- bool) {
	// TODO implement logic to remove a worker only after all requests have finished processing

	// Delete the last droplet
	toDelete := m.droplets[len(m.droplets)-1]
	if _, err := m.doClient.Droplets.Delete(toDelete.ID); err != nil {
		utils.Die("Error deleting droplet: %s", err.Error())
	}

	m.droplets = m.droplets[0 : len(m.droplets)-1]

	c <- true
}

func (m *Master) writeAddresses() {
	var (
		file *os.File
		temp *template.Template
		err  error
	)

	if temp, err = template.ParseFiles(m.balanceConfigTemplate); err != nil {
		utils.Die("Error reading in template: %s", err.Error())
	}
	if file, err = os.OpenFile(m.balanceConfigFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm); err != nil {
		utils.Die("Error opening load balancer config file: %s", err.Error())
	}

	// Get the IP addresses together
	ips := make(map[string]string)
	for _, droplet := range m.droplets {
		for _, addr := range droplet.Networks.V4 {
			if addr.Type == "public" {
				ips[droplet.Name] = addr.IPAddress
				break
			}
		}
	}

	// Write changes out to the template file
	if err = temp.Execute(file, ips); err != nil {
		utils.Die("Error writing template out to the file: %s", err.Error())
	}
}

func (m *Master) reload() {
	var (
		out []byte
		err error
	)

	if out, err = exec.Command("sh", "-c", m.command).Output(); err != nil {
		utils.Die("Error executing 'reload' command: %s", err.Error())
	}
	fmt.Printf("Executed command. Output: '%s'\n", strings.TrimSpace(string(out)))
}

func (m *Master) streamStats() {
	m.statsdClientBuffer.FGauge("loadavg", m.currentLoadAvg)
	m.statsdClientBuffer.Gauge("workers", int64(len(m.droplets)))
	fmt.Println("Streamed to statsd")
	time.Sleep(time.Second * 5)
}

func (m *Master) MonitorWorkers() {
	// Send out survey requests indefinitely
	workerQuery := make(chan float64)
	dropletCreatePoll := make(chan *godo.Droplet)
	dropletDeletePoll := make(chan bool)

	// Start querying the worker threads
	go m.queryWorkers(workerQuery)

	// Start streaming stats if needed
	if m.statsdClientBuffer != nil {
		go m.streamStats()
	}

	for {
		select {
		case loadAvg := <-workerQuery:
			fmt.Printf("Load avg: %f\n", loadAvg)
			m.currentLoadAvg = loadAvg

			// Make scaling decision
			if m.shouldAddWorker(loadAvg) {
				fmt.Println("Max threshold met")
				m.waitingOnWorkerChange = true
				go m.addWorker(dropletCreatePoll)
			} else if m.shouldRemoveWorker(loadAvg) {
				fmt.Println("Min threshold met")
				m.waitingOnWorkerChange = true
				go m.removeWorker(dropletDeletePoll)
			}

		case newDroplet := <-dropletCreatePoll:
			go m.cooldown()
			m.waitingOnWorkerChange = false

			// Add the new droplet to the list
			m.droplets = append(m.droplets, *newDroplet)

			// Write it to the config file and execute the "reload" command
			m.writeAddresses()
			m.reload()

		case <-dropletDeletePoll:
			go m.cooldown()
			m.waitingOnWorkerChange = false
			m.writeAddresses()
			m.reload()

		}
	}
}

func (m *Master) CleanUp() {
	m.statsdClientBuffer.Close()
}

type WorkerConfig struct {
	NamePrefix   string   `json:"namePrefix"`
	DropletNames []string `json:"dropletNames"`
}
