#!/bin/sh
set -e

# Initialize if needed (for volume mount scenarios)
if [ ! -f /root/.beatoz/config/genesis.json ]; then
  echo 'Config not found. Initializing beatoz...'
  beatoz init --chain_id 0x1234 --home /root/.beatoz --assumed_block_interval 1s
  echo 'Initialization complete.'
fi

# Display wallet information
echo '===================================================='
echo '=== Wallet Keys ==='
echo '=== DO NOT USE THESE PRIVATE KEYS ON MAINNET !!! ==='
echo '===================================================='
for file in /root/.beatoz/walkeys/wk*.json; do
  echo ''
  grep address "$file" | sed 's/.*address/  Address       /' | sed 's/[",:]//g' | awk '{$NF = "0x" tolower($NF); print}'
  beatoz wallet-key "$file" | grep prvKey | sed 's/.*prvKey/  PrivateKey /' | sed 's/[",:]//g' | awk '{$NF = "0x" tolower($NF); print}'
  echo ''
done
echo ' '
echo '===================================================='
echo '===                                              ==='
echo '=== DO NOT USE THESE PRIVATE KEYS ON MAINNET !!! ==='
echo '===                                              ==='
echo '===================================================='
echo ''
echo 'ChainID: 0x1234'
nodeId=$(beatoz show-node-id --home /root/.beatoz)
echo "NodeID:  $nodeId"
echo ''
echo "Starting beatoz $(beatoz version) ..."

# Execute beatoz with passed arguments
exec beatoz "$@"