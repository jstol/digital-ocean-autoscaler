#!/usr/bin/env bash
set -ex

PROG="autoscaler-master"
addr=$1
cmd=$2

if [ -z "${addr}" ] ; then
	echo "Usage: run_master.bash [HOST:PORT]"
	exit
fi

if [ -z "${cmd}" ] ; then
	# cmd="service haproxy reload"
	cmd="echo ADDED NODE"
fi

cleanup() {
	rm ${PROG}
}
trap cleanup EXIT

go build -o ${PROG} ./autoscaler

./${PROG} -host ${addr} -command "${cmd}" -template "x" -config "y" -overloaded 0.15 -underused 0.1 -min 1 -max 1
