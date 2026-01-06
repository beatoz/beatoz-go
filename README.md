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

## Docker

### Build Docker Image

Build the Docker image with automatic version tagging:

```bash
make docker-build
```

This will create three tagged images:
- `beatoz-re:latest`
- `beatoz-re:v1.12.0` (version from git tag)
- `beatoz-re:v1.12.0-c842f85` (version + git commit hash)

### Run with Docker

**Pull Image from Docker Hub:**

```bash
docker pull beatoz/beatoz-re:latest
```

**Note**: If you already have the image locally, `docker run` will use the local image. To get the latest version, run `docker pull` first.

**Quick Start (ephemeral):**

Run without data persistence (container data is lost on restart):

```bash
docker run -d \
  --name beatoz-node \
  -p 26656:26656 \
  -p 26657:26657 \
  -p 26658:26658 \
  --restart unless-stopped \
  beatoz/beatoz-re:latest
```

**With Data Persistence:**

Run with volume mount to persist blockchain data:

```bash
docker run -d \
  --name beatoz-node \
  -p 26656:26656 \
  -p 26657:26657 \
  -p 26658:26658 \
  -v ./beatoz-docker:/root/.beatoz \
  --restart unless-stopped \
  beatoz/beatoz-re:latest
```

**View Logs:**

```bash
docker logs -f beatoz-node
```

**Stop and Remove:**

```bash
docker stop beatoz-node
docker rm beatoz-node
```

### Docker Configuration

The Docker setup includes:

- **Pre-initialized configuration**: The image contains pre-generated genesis.json, config.toml, and wallet keys
- **Automatic wallet display**: Wallet addresses and private keys are displayed on startup
- **Chain ID**: `0x1234` (for testing only)
- **Ports**:
  - `26656`: P2P port
  - `26657`: RPC port
  - `26658`: Monitor port

**Environment Variables:**

The following environment variables are set in the Docker image:

- `BEATOZ_VALIDATOR_SECRET`: Password for validator key (default: "unsafe_password")
- `BEATOZ_HOLDER_SECRET`: Password for holder accounts (default: "unsafe_password")
- `BEATOZ_WALKEY_SECRET`: Password for wallet keys (default: "unsafe_password")

**⚠️ WARNING**: The pre-generated keys are for testing only. **DO NOT USE THESE KEYS ON MAINNET!**
