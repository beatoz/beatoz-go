#!/usr/bin/env bash
if [[ "$1" != "" ]]; then
  OPT="--p2p.persistent_peers ${1}"
  echo "start option is ${OPT}"
fi
nohup ~/bin/beatoz start $OPT --priv_validator_secret 1 > ~/bin/log 2>&1 &