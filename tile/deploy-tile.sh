#!/bin/bash

set -e

TILE_DIR=$1
METADATA_FILE=$2

cp $METADATA_FILE .
ls
echo $TILE_DIR
ls ${TILE_DIR}
TILE_FILE=`cd "${TILE_DIR}"; ls *.pivotal`
if [ -z "${TILE_FILE}" ]; then
	echo "No files matching ${TILE_DIR}/*.pivotal"
	ls -lR "${TILE_DIR}"
	exit 1
fi
PRODUCT=`echo "${TILE_FILE}" | sed "s/-[^-]*$//"`
VERSION=`echo "${TILE_FILE}" | sed "s/.*-//" | sed "s/\.pivotal\$//"`

PCF=pcf
APP_DOMAIN=`$PCF cf-info | grep apps_domain | cut -d" " -f3`

echo "Uploading ${TILE_FILE}"
$PCF import "${TILE_DIR}/${TILE_FILE}"
echo

echo "Installing product ${PRODUCT} version ${VERSION}"
$PCF install "${PRODUCT}" "${VERSION}"
echo

echo "Configuring product ${PRODUCT}"
$PCF configure "${PRODUCT}"
echo

echo "Applying Changes"
$PCF apply-changes --deploy-errands=deploy-all
echo
