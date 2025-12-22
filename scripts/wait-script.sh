#!/usr/bin/env bash

IFS="," read -ra PORTS <<<"$WAIT_PORTS"
path=$(dirname "$0")

PIDs=()
for port in "${PORTS[@]}"; do
  (
    timeout=180
    elapsed=0
    while [ $elapsed -lt $timeout ]; do
      if curl -f -s "http://localhost:$port/manage/health" > /dev/null 2>&1; then
        echo "Host localhost:$port is active"
        exit 0
      fi
      sleep 2
      elapsed=$((elapsed + 2))
    done
    echo "Operation timed out for port $port" >&2
    exit 1
  ) &
  PIDs+=($!)
done

for pid in "${PIDs[@]}"; do
  if ! wait "${pid}"; then
    exit 1
  fi
done
