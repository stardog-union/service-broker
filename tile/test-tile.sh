#!/usr/bin/env bash

set -e

export STARDOG_SERVICE_REPO_DIR=$1
export STARDOG_URL=$2
export STARDOG_PW=$3
ORG=$4
SPACE=$5
METADATA_FILE=$6

cp $METADATA_FILE .
echo "Testing the tile..."

echo "pcf target -o $ORG -s $SPACE"
pcf target -o $ORG -s $SPACE

export CF_DOMAIN_NAME=`pcf cf-info | grep apps_domain | cut -d" " -f3`

set +e
python ${STARDOG_SERVICE_REPO_DIR}/tile/test.py
rc=$?
pcf target -o $ORG -s $SPACE
exit $rc
