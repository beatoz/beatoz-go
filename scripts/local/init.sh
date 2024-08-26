#!/usr/bin/env bash

. ./env.sh


# init
idx=0
for home in "${NODE_HOMES[@]}"; do
  echo ""
  read -p "initialize beatoz at ${home} ?" a
  echo ""

  rm -rf ${home}
  $EXE init --home "${home}" --chain_id localnet --priv_validator_secret 1
  nodeid=`$EXE show-node-id --home "${home}"`
  wk=`$EXE show-wallet-key --secret 1 "${home}/config/priv_validator_key.json"`
  addr=`echo "${wk}" | grep "address" | egrep -o '[A-Fa-f0-9]{40,}'`
  prvk=`echo "${wk}" | grep "PrvKey" | egrep -o '[A-Fa-f0-9]{40,}'`
  echo "nodeid:  ${nodeid}"
  echo "address: ${addr}"
  echo "prvkey:  ${prvk}"

  if [[ "$idx" -gt "0" ]]; then
    echo "copy genesis file... ${NODE_HOMES[0]}/config/genesis.json  to $home/config/genesis.json"
    cp -f ${NODE_HOMES[0]}/config/genesis.json $home/config/genesis.json
  fi
  idx=$((idx+1))
done