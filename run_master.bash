#!/usr/bin/env bash
set -ex

PROG="autoscaler-master"
addr=$1
token=$2
slug=$3

if [ -z "${addr}" ] || [ -z "${token}" ] || [ -z "${slug}" ] ; then
	echo "Usage: run_master.bash [HOST:PORT] [TOKEN] [IMAGE SLUG]"
	exit
fi

cleanup() {
	rm ${PROG}
}
trap cleanup EXIT

go build -o ${PROG} ./autoscaler

./${PROG} -host=${addr} -token="${token}" -image="${slug}" \
	-command="service haproxy reload" \
	-balancetemplate="autoscaler/haproxy-template.cfg" \
	-balanceconfig="/etc/haproxy/haproxy.cfg" \
	-workerconfig="autoscaler/config/config.json" \
	-overloaded=0.65 -underused=0.2 \
	-min=10 -max=20 \
	-statsd=true -statsdaddr="localhost:8125" \
	-weights=true -autoscale=true
