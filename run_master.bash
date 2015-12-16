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

./${PROG} -host ${addr} -token "${token}" -image "${slug}" \
	-command "service haproxy reload" \
	-balancetemplate "/root/autoscaler-template.cfg" \
	-balanceconfig "/etc/haproxy/haproxy.cfg" \
	-workerconfig "autoscaler/config/config.json" \
	-overloaded 0.3 -underused 0.1 \
	-min 1 -max 3 \
	-statsdaddr "localhost:8125"
