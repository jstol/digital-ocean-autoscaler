# digital-ocean-autoscaler
An autoscaler for Digital Ocean droplets that sits on top of HAProxy.

This tool runs a "master" process on the same droplet as HAProxy, with "client" processes running on app droplets.

The clients share CPU load metrics (`loadavg`) with the master, which then in turn dynamically sets the load balancing weights of the app servers and adds/removes droplets as needed.
