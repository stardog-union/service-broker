#!/bin/bash

set -e

bin=$(dirname $0)

go build -o $bin/../stardog-service-broker
