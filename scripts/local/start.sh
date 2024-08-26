#!/usr/bin/env bash

. ./env.sh

# start nodes
rpcport=26657
port=36656
nodeid=""
peer_opt="" #--p2p.persistent_peers ${nodeid}@${ip}:${port}
for home in "${NODE_HOMES[@]}"; do
  echo ""
  read -p "start beatoz at ${home} ?" a
  echo ""

  echo "this peer will be connecto to $peer_opt"

  nohup ${EXE} start --home "${home}" \
  --p2p.laddr "tcp://0.0.0.0:${port}" \
  ${peer_opt} \
  --priv_validator_secret "1" \
  --rpc.laddr "tcp://0.0.0.0:${rpcport}" \
  --rpc.cors_allowed_origins "*" \
  1>${home}/log 2>&1 &

  nodeid=`$EXE show-node-id --home "${home}"`
  peer_opt="--p2p.persistent_peers ${nodeid}@127.0.0.1:${port}"
  port=$((port+1))
  rpcport=$((rpcport+1))
done
