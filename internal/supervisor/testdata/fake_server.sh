#!/bin/sh
echo $$ > "$1"
trap 'exit 0' TERM
while true; do sleep 0.1; done
