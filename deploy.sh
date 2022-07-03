#!/bin/bash
set -euo pipefail

SOURCE_PATH="$GOPATH/src/fsf/tx"
DEST_USER=fsf
DEST_TX_HOST=tx
DEST_OUT_HOST=jmp
DEST_PARENT=/home/fsf/go/src/fsf
DEST_PATH="$DEST_PARENT/tx"

echo "Building"
echo -n "arm-v6..."
GOMAXPROCS=1 GOOS=linux GOARCH=arm GOARM=6 go build -o tx-arm6 .
echo "done"
echo -n "arm-v7..."
GOMAXPROCS=1 GOOS=linux GOARCH=arm GOARM=7 go build -o tx-arm7 .
echo "done"

echo -n "Transferring to $DEST_TX_HOST.."
rsync -r \
  --exclude="tx/.git*" \
  --exclude="tx/*.bak" \
  --exclude="tx/tx" \
  --exclude="tx/tx-arm7" \
  "$SOURCE_PATH" "$DEST_USER"@"$DEST_TX_HOST":"$DEST_PARENT"
echo "done"

echo -n "Transferring to $DEST_OUT_HOST.."
rsync -r \
  --exclude="tx/.git*" \
  --exclude="tx/*.bak" \
  --exclude="tx/tx" \
  --exclude="tx/tx-arm6" \
  "$SOURCE_PATH" "$DEST_USER"@"$DEST_OUT_HOST":"$DEST_PARENT"
echo "done"

echo -n "Setting capabilities..."
ssh tx "$DEST_PATH/setcap.sh"
echo "done"

echo -n "Halting..."
ssh $DEST_TX_HOST sudo systemctl stop radio
ssh $DEST_OUT_HOST sudo systemctl stop radio
echo "Done"

echo -n "Starting..."
ssh $DEST_TX_HOST sudo systemctl start radio
ssh $DEST_OUT_HOST sudo systemctl start radio
