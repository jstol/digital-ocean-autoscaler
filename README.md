# digital-ocean-autoscaler
A horizontal autoscaler for applications deployed to DigitalOcean. The tool can be used alongside HAProxy to augment its weighted round robin load balancing algorithm. Weights can be dynamically set to redirect requests to the least loaded worker Droplet.

This tool runs a "node manager" process (on the same droplet as HAProxy), with "worker monitor" processes running on app Droplets. Worker monitor processes share CPU load metrics (`loadavg`) with the node manager, which then in turn adds/removes Droplets as needed (and dynamically sets HAProxy's weights for each of the app server Droplets).

Node managers communicate with worker monitors through a nanomsg `SURVEY` socket (using the [Mangos](https://github.com/go-mangos/mangos) library).
