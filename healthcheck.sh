#!/bin/sh

THRESHOLD_KB=100000

MEMORY_USAGE_KB=$(awk '/VmRSS/ {print $2}' /proc/1/status)

if [ $MEMORY_USAGE_KB -gt $THRESHOLD_KB ]; then
    echo "Memory usage ($MEMORY_USAGE_KB KB) exceeds threshold ($THRESHOLD_KB KB)"
    exit 1
else
    echo "Memory usage OK: $MEMORY_USAGE_KB KB"
    exit 0
fi
