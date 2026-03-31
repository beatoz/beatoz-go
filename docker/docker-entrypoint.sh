#!/bin/sh
set -e

CHAIN_ID="0x1234"

# Initialize if needed (for volume mount scenarios)
if [ ! -f /root/.beatoz/config/genesis.json ]; then
  echo 'Config not found genesis.json. Initializing beatoz...'
  beatoz init --chain_id ${CHAIN_ID} --home /root/.beatoz --consensus.create_empty_blocks_interval 60s
  echo 'Initialization complete.'
fi

# Display wallet information
echo '===================================================='
echo '=== Wallet Keys ==='
echo '=== DO NOT USE THESE PRIVATE KEYS ON MAINNET !!! ==='
echo '===================================================='
echo ''
for file in /root/.beatoz/walkeys/wk*.json; do
  beatoz wallet-key "$file" | grep -v 'wallet file' | awk -F': ' '{print $1 ": 0x" tolower($2)}'
  echo ''
done
echo '===================================================='
echo '===                                              ==='
echo '=== DO NOT USE THESE PRIVATE KEYS ON MAINNET !!! ==='
echo '===                                              ==='
echo '===================================================='
echo ''
echo "ChainID: ${CHAIN_ID}"
nodeId=$(beatoz show-node-id --home /root/.beatoz)
echo "NodeID:  $nodeId"
echo ''
echo "Starting beatoz $(beatoz version) ..."

# Execute beatoz with passed arguments
exec beatoz "$@"