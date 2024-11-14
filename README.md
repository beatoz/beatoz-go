# BEATOZ Blockchain

## Prerequisite

### golang
Install golang v1.21+

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

### protoc-gen-go-grpc
```bash
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
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
build/darwin/beatoz init --chain_id local_test_net --priv_validator_secret 1234

...

build/darwin/beatoz start
```

* BEATOZ's root directory is `$HOME/.beatoz`.
* BEATOZ's config file is `$HOME/.beatoz/config/config.toml`.
* Genesis file is `$HOME/.beatoz/config/gebesus.json`.
* Validator private key file is  `$HOME/.beatoz/config/priv_validator_key.json`.
* Initial wallet files are in `$HOME/.beatoz/walkeys/`.
* To show private key, run `build/darwin/beatoz show-wallet-key {Wallet Key Files}`.  
  (e.g. run `build/darwin/beatoz show-wallet-key ~/.beatoz/config/priv_validator_key.json`)