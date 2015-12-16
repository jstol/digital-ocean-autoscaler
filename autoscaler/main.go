package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/jstol/digital-ocean-autoscaler/autoscaler/master"
	"github.com/jstol/digital-ocean-autoscaler/utils"
)

func main() {
	host := flag.String("host", "0.0.0.0:8000", "the IP address and port to bind to")
	command := flag.String("command", "", "the command to run after writing out the load balancer's new configuration file")
	balanceConfigTemplate := flag.String("balancetemplate", "", "the load balancer config file template to use")
	balanceConfigFile := flag.String("balanceconfig", "", "the load balancer config file to write to")
	workerConfigFile := flag.String("workerconfig", "", "the worker config file (JSON) to read from")
	digitalOceanToken := flag.String("token", "", "the Digital Ocean API token to use")
	digitalOceanImageID := flag.String("image", "", "the ID of the image to use when creating worker nodes")
	overloadedCpuThreshold := flag.Float64("overloaded", 0.7, "the average CPU usage threshold after which the nodes are considered overloaded")
	underusedCpuThreshold := flag.Float64("underused", 0.3, "the CPU usage threshold to consider a node as underutilized")
	minWorkers := flag.Int64("min", 1, "the minimum number of workers to have")
	maxWorkers := flag.Int64("max", 10, "the minimum number of workers to have")
	streamStatsd := flag.Bool("statsd", false, "a flag indicating whether or not to stream statsd stats")
	statsdAddr := flag.String("statsdaddr", "", "the address and port of the statsd server")
	statsdPrefix := flag.String("statsdprefix", "autoscaler.", "the statsd prefix to use")
	statsdInterval := flag.Int64("statsdinterval", 2, "the number of seconds to wait before flushing every batch of statsd stats")
	pollInterval := flag.Int64("pollinterval", 3, "the amount of time (in seconds) to wait between polling Digital Ocean for updates")
	cooldownInterval := flag.Int64("cooldowninterval", 15, "the amount of time (in seconds) to wait before making changes to workers after altering the worker set")
	surveyDeadline := flag.Int64("surveydeadline", 1, "the amount of time (in seconds) to wait to receive feedback from workers")
	queryInterval := flag.Int64("surveytimeout", 3, "the amount of time (in seconds) to leave between querying workers")
	flag.Parse()

	// Handle checking command line arguments
	if *command == "" {
		utils.Die("Missing -command flag")
	} else if *balanceConfigTemplate == "" {
		utils.Die("Missing -balancetemplate flag")
	} else if *balanceConfigFile == "" {
		utils.Die("Missing -balanceconfig flag")
	} else if *workerConfigFile == "" {
		utils.Die("Missing -workerconfig flag")
	} else if *digitalOceanToken == "" {
		utils.Die("Missing -token flag")
	} else if *digitalOceanImageID == "" {
		utils.Die("Missing -image flag")
	} else if *minWorkers <= 0 {
		utils.Die("The -min must be non-negative")
	} else if *maxWorkers <= 0 {
		utils.Die("The -max must be non-negative")
	} else if *maxWorkers < *minWorkers {
		utils.Die("Max number of workers must be greater than or equal to the min")
	} else if *streamStatsd && *statsdAddr == "" {
		utils.Die("Statsd streaming requested, but missing -statsdaddr flag")
	}

	// Read in the config file
	var workerConfig master.WorkerConfig

	jsonData, err := ioutil.ReadFile(*workerConfigFile)
	if err != nil {
		utils.Die("Error reading in config file: %s", err.Error())
	}
	if err = json.Unmarshal(jsonData, &workerConfig); err != nil {
		utils.Die("Error parsing JSON in config file: %s", err.Error())
	}

	// Start the master
	fmt.Printf("Starting master at %s\n", *host)
	var monitor *master.Master

	if !*streamStatsd {
		monitor = master.NewMaster(
			*host,
			&workerConfig,
			*command,
			*balanceConfigTemplate, *balanceConfigFile,
			*digitalOceanToken, *digitalOceanImageID,
			*overloadedCpuThreshold, *underusedCpuThreshold,
			*minWorkers, *maxWorkers,
			time.Duration(*pollInterval)*time.Second, time.Duration(*cooldownInterval)*time.Second, time.Duration(*surveyDeadline)*time.Second, time.Duration(*queryInterval)*time.Second,
		)
	} else {
		monitor = master.NewMasterWithStatsd(
			*host,
			&workerConfig,
			*command,
			*balanceConfigTemplate, *balanceConfigFile,
			*digitalOceanToken, *digitalOceanImageID,
			*overloadedCpuThreshold, *underusedCpuThreshold,
			*minWorkers, *maxWorkers,
			time.Duration(*pollInterval)*time.Second, time.Duration(*cooldownInterval)*time.Second, time.Duration(*surveyDeadline)*time.Second, time.Duration(*queryInterval)*time.Second,
			*statsdAddr, *statsdPrefix, time.Duration(*statsdInterval)*time.Second,
		)
	}
	defer monitor.CleanUp()

	monitor.MonitorWorkers()
}
