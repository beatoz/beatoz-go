# testnet0 (chainId: `0xbea701`)

### Genesis
[genesis.json](./config/genesis.json)

### SENTRY Nodes

- 604afe9f6d03599ed0b3f9cd06b708ad43145545@3.34.46.70:26656
- e0c7f3caf056f3fbf33dc88e8ff549d2e258b3ef@54.180.100.243:26656
- c35e389ac939990b7733652eaac0c544e3d9176b@13.209.17.110:26656
- 1e8f9b788b63fff00c515b1f89a51febc61346ce@3.36.57.54:26656
- 3b6b4fd4853393e9de2bdcf3d1b811cf78fdeb6e@54.180.235.214:26656

### RPC

https://rpc-testnet0.beatoz.io

### Connect to `testnet0` network

1. init
```sh
beatoz init --home /your/testnet0/directory {other_options}
```

2. download and copy [genesis.json](genesis.json)
```
cp -f /your/download/directory/genesis.json /your/testnet0/directory/config/genesis.json
```

3. start beatoz
```
beatoz start --home /your/testnet0/directory \
--p2p.persistent_peers \
604afe9f6d03599ed0b3f9cd06b708ad43145545@3.34.46.70:26656,\
e0c7f3caf056f3fbf33dc88e8ff549d2e258b3ef@54.180.100.243:26656,\
c35e389ac939990b7733652eaac0c544e3d9176b@13.209.17.110:26656,\
1e8f9b788b63fff00c515b1f89a51febc61346ce@3.36.57.54:26656,\
3b6b4fd4853393e9de2bdcf3d1b811cf78fdeb6e@54.180.235.214:26656 \
{other_options}
```