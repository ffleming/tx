#!/bin/bash
set -euo pipefail

SOURCE_PATH="$GOPATH/src/fsf/tx"
DEST_USER=fsf
DEST_HOST=tx
DEST_PARENT=/home/fsf/go/src/fsf
DEST_PATH="$DEST_PARENT/tx"

echo -n "Building..."
GOMAXPROCS=1 GOOS=linux GOARCH=arm GOARM=6 go build -o tx-arm .
echo "done"

echo -n "Transferring..."
rsync -r \
  --exclude="tx/.git*" \
  --exclude="tx/*.bak" \
  --exclude="tx/tx" \
  "$SOURCE_PATH" "$DEST_USER"@"$DEST_HOST":"$DEST_PARENT"
echo "done"

echo -n "Setting capabilities..."
ssh tx "$DEST_PATH/setcap.sh"
echo "done"

echo -n "Halting..."
ssh tx sudo systemctl stop radio
echo "Done"

echo -n "Starting..."
ssh tx sudo systemctl start radio
