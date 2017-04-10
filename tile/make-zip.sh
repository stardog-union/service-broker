#!/bin/bash

set -e

bin=$(dirname $0)
cd $bin/../
basedir=$(pwd)


zip -r tile/stardog-service-broker.zip Godeps README.md LICENSE main.go server_test.go manifest.yml store testapp plans testdata broker tile/conf.cf.json vendor resources

