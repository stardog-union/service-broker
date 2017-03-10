#!/bin/bash

set -e

here=$(dirname $0)
cd $here
cd ..

go test github.com/stardog-union/service-broker/broker
go test github.com/stardog-union/service-broker/plans/shared

go test -v -cover -coverpkg github.com/stardog-union/service-broker/broker,github.com/stardog-union/service-broker/plans/shared -coverprofile=coverage.out

