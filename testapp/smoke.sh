#!/bin/bash

cd `dirname $0`

function clean_up() {
    cf delete -f vcap-echo
    cf delete-service -f stardog-service
    cf delete -f stardog-service-broker
    cf delete-service-broker -f stardogbroker
}

if [ -z $CF_TESTING_STARDOG_URL ]; then
    echo "The env CF_TESTING_STARDOG_URL must be set to a Stardog service URL"
    exit 1
fi

clean_up
set -e
cd ..
sed "s^@@SD_URL@@^$CF_TESTING_STARDOG_URL^g" data/conf.json.example > data/conf.json
cf push
cf create-service-broker stardogbroker someuser somethingsecure http://stardog-service-broker.bosh-lite.com

cf enable-service-access stardog
cf marketplace
cf create-service Stardog shareddb stardog-service
cd testapp
cf push
curl -v http://vcap-echo.bosh-lite.com

clean_up
