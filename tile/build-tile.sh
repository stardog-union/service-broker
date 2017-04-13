#!/bin/bash

set -e

REPO_DIR=$1
TILE_HISTORY=$2

cd $REPO_DIR
cd tile
echo "Make the zip file..."
./make-zip.sh
cd ../..
echo "List the history files..."
ls $TILE_HISTORY/
cp $TILE_HISTORY/tile-history-*.yml build-dir/tile-history.yml
echo "Using history data..."
cat build-dir/tile-history.yml
cp stardog-broker-repo//tile/* build-dir/
cd build-dir
echo "Build the tile..."
tile build
echo "Manage output..."
VERSION=`grep '^version:' tile-history.yml | sed 's/^version: //'`
HISTORY="tile-history-${VERSION}.yml"
echo "New history version $VERSION"
mv tile-history.yml tile-history-${VERSION}.yml

cat tile-history-${VERSION}.yml

echo "Done"
