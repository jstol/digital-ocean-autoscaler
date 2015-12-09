package main

import (
	"flag"
	"fmt"

	"github.com/jstol/digital-ocean-autoscaler/autoscaler/master"
	"github.com/jstol/digital-ocean-autoscaler/utils"
)

func main() {
	host := flag.String("host", "0.0.0.0:8000", "the IP address and port to bind to")
	command := flag.String("command", "", "the command to run after writing out the load balancer's new configuration file")
	configTemplate := flag.String("template", "", "the load balancer config file template to use")
	configFile := flag.String("config", "", "the load balancer config file to write to")
	overloadedCpuThreshold := flag.Float64("overloaded", 0.7, "the average CPU usage threshold after which the nodes are considered overloaded")
	underusedCpuThreshold := flag.Float64("underused", 0.3, "the CPU usage threshold to consider a node as underutilized")
	minNodes := flag.Int64("min", 1, "the minimum number of nodes to have")
	maxNodes := flag.Int64("max", -1, "the minimum number of nodes to have")
	flag.Parse()

	// Handle checking command line arguments
	if *command == "" {
		utils.Die("Missing -command flag")
	} else if *configTemplate == "" {
		utils.Die("Missing -template flag")
	} else if *configFile == "" {
		utils.Die("Missing -file flag")
	} else if *minNodes <= 0 {
		utils.Die("The -min must be non-negative")
	} else if *maxNodes <= 0 {
		utils.Die("The -max must be non-negative")
	} else if *maxNodes < *minNodes {
		utils.Die("Max number of nodes must be greater than or equal to the min")
	}

	fmt.Printf("Starting master at %s\n", *host)
	monitor := master.NewMaster(*host, *command, *configTemplate, *configFile, *overloadedCpuThreshold, *underusedCpuThreshold, *minNodes, *maxNodes)
	monitor.MonitorNodes()
}
