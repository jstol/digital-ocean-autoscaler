#!/usr/bin/env bash
set -ex

PROG="autoscaler-master"
addr=$1

if [ -z "${addr}" ] ; then
	echo "Usage: run_master.bash [HOST:PORT]"
	exit
fi

cleanup() {
	rm ${PROG}
}
trap cleanup EXIT

go build -o ${PROG} ./master

./${PROG} -host ${addr}
