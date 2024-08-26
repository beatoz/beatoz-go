#!/usr/bin/env bash

PID=`ps -ef | grep 'beatoz' | grep -v grep | awk '{print $2}'`
if [[ -n "$PID" ]]; then
  echo "Stopping kms-go (PID:${PID}) ..."
  kill -15 ${PID}
fi