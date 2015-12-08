#!/usr/bin/env bash
set -ex

PROG="autoscaler-client"

cleanup() {
	rm ${PROG}
}
trap cleanup EXIT

go build -o ${PROG} ./client

./${PROG}
