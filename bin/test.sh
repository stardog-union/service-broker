#!/bin/bash

set -e

here=$(dirname $0)
cd $here
cd ..

go test github.com/stardog-union/service-broker/broker
go test github.com/stardog-union/service-broker/plans
go test github.com/stardog-union/service-broker/plans/shared
go test github.com/stardog-union/service-broker/store

go test -v -cover -coverpkg github.com/stardog-union/service-broker/broker,github.com/stardog-union/service-broker/plans/shared,github.com/stardog-union/service-broker/plans/perinstance,github.com/stardog-union/service-broker/store/stardog,github.com/stardog-union/service-broker/store/sql -coverprofile=coverage.out
