# BEATOZ Blockchain

## Prerequisite

### golang

Install golang v1.23+

### protoc (protobuf compiler)

```bash
brew install protobuf

...

protoc --version
libprotoc 3.21.12
```

### protoc-gen-go

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go

...

protoc-gen-go --version
protoc-gen-go v1.28.1
```

## Sources

```bash
git clone https://github.com/beatoz/beatoz-go.git
```

## Build

```bash
make
```

## Run BEATOZ

```bash
build/darwin/beatoz init --chain_id 1234 --priv_validator_secret "mypassword"

...

build/darwin/beatoz start
```

* BEATOZ's root directory is `$HOME/.beatoz`.
* BEATOZ's config file is `$HOME/.beatoz/config/config.toml`.
* Genesis file is `$HOME/.beatoz/config/gebesus.json`.
* Validator private key file is  `$HOME/.beatoz/config/priv_validator_key.json`.
* Initial wallet files are in `$HOME/.beatoz/walkeys/`.
* To show private key, run `build/darwin/beatoz wallet-key {Wallet Key Files}`.
  (e.g. run `build/darwin/beatoz wallet-key ~/.beatoz/config/priv_validator_key.json`)

If you want to participate in the network `testnet0(chainId:0xbea701` of BEATOZ, refer to [testnet0](docs/testnet0/README.md).

## Docker

Pre-built Docker images are available on Docker Hub. For detailed instructions on running BEATOZ with Docker, please visit:

https://hub.docker.com/r/beatoz/beatoz-re

## API Documentation

Interactive API documentation with Swagger UI is available online. You can explore all available RPC endpoints, request/response schemas, and try out API calls directly:

https://beatoz.github.io/beatoz-go/swagger
