#!/bin/bash
set -euo pipefail
GOMAXPROCS=1 GOOS=linux GOARCH=arm GOARM=6 go build -o tx-arm .
ssh tx killall tx-arm || echo "process not running"
scp -r ~/go/src/fsf/tx tx:go/src/fsf && echo "copied"
ssh tx ~/go/src/fsf/tx/setcap.sh && echo "capabilities set"
