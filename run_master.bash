#!/usr/bin/env bash
set -ex

PROG="autoscaler-master"

cleanup() {
	rm ${PROG}
}
trap cleanup EXIT

go build -o ${PROG} ./master

./${PROG}
