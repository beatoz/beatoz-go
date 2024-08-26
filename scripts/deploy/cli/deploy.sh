#!/usr/bin/env bash

#make build
#make build-deploy
#build/darwin/beatoz init --chain_id demonet --priv_validator_secret 1

NODES=("3.38.221.227" "3.37.191.127" "3.34.201.6" "15.165.45.176" "15.165.38.111")
IDX=0

for N in "${NODES[@]}"; do
  T="ubuntu@${N}"
  echo Stop ${T}...
  ssh -i ~/.ssh/anode-dev.pem ${T} "chmod +x ~/bin/*.sh; ~/bin/stop.sh"
done


# init locally
rm -rf ~/.anode
build/linux/anode init --chain_id demonet --priv_validator_secret 1
cp -f scripts/config/config.toml ~/.anode/config/config.toml
WKEY=($(ls ~/.anode/walkeys/wk*))

PRE=""
for N in "${NODES[@]}"; do
  T="ubuntu@${N}"

  echo " "
  echo " "
  echo " "
    echo "*********** Deploy to ${T}"
  # upload and ...
  scp -i ~/.ssh/anode-dev.pem .deploy/deploy.gz.tar ${T}:~/
  ssh -i ~/.ssh/anode-dev.pem ${T} "mkdir -p ~/bin; tar -xzvf ~/deploy.gz.tar -C ~/bin; chmod +x ~/bin/*.sh"

  if [[ "0" == "$IDX" ]]; then
#    echo " "
#    echo "************* Copy priv_validator_key.json"
#    scp -i ~/.ssh/beatoz-dev.pem ~/.beatoz/config/priv_validator_key.json ${T}:~/.beatoz/config/priv_validator_key.json
    echo "First node is current node"
  else
  # init & configuration
    ssh -i ~/.ssh/anode-dev.pem ${T} "rm -rf ~/.beatoz"
    ssh -i ~/.ssh/anode-dev.pem ${T} "~/bin/init.sh"
    scp -i ~/.ssh/anode-dev.pem ~/.anode/config/config.toml ${T}:~/.anode/config/config.toml
    scp -i ~/.ssh/anode-dev.pem ~/.anode/config/genesis.json ${T}:~/.anode/config/genesis.json

    echo " "
    echo "************* Copy ${WKEY[$IDX]}"
    scp -i ~/.ssh/anode-dev.pem ${WKEY[$IDX]} ${T}:~/.anode/config/priv_validator_key.json
  fi
  ssh -i ~/.ssh/anode-dev.pem ${T} "~/bin/reset.sh"
  nodeid=`ssh  -i ~/.ssh/anode-dev.pem ${T} "~/bin/beatoz show-node-id"`
  echo " "
  echo "Start ...."

  ssh -i ~/.ssh/anode-dev.pem ${T} "~/bin/start.sh ${PREN}"

  echo "***************************************************"
  echo " "
  PREN="${nodeid}@${N}:26656"
  IDX=$((IDX+1))
done