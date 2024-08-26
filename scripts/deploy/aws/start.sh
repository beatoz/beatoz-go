#!/usr/bin/env bash

BEATOZ_DATADIR="$HOME/.beatoz"
BEATOZ_LOG="${BEATOZ_DATADIR}/log"
BEATOZ_BINFILE="$HOME/beatoz-deploy/beatoz"
BEATOZ_INITDIR="$HOME/beatoz-deploy/.beatoz"


if [ "$DEPLOYMENT_GROUP_NAME" == "beatoz-dev-init-dg" ]; then
  #
  # Remove ~/.beatoz and copy ~//beatoz-deploy/.beatoz to ~/.beatoz
  #
  if [[ -d "$BEATOZ_DATADIR" ]]; then
    echo "Remove ${BEATOZ_DATADIR} ..."
    rm -rf ${BEATOZ_DATADIR}
  fi

  echo "Copy '${BEATOZ_INITFILES}' to '${BEATOZ_DATADIR}' ..."

  # copy ./.beatoz to ~/.beatoz
  cp -rf ${BEATOZ_INITFILES} ${BEATOZ_DATADIR}

elif [ "$DEPLOYMENT_GROUP_NAME" == "beatoz-dev-reset-dg" ]; then
  #
  # Reset ~/.beatoz
  # Remove blockchain data.
  #
  echo "Reset data in ${BEATOZ_DATADIR}..."
  ${BEATOZ_BINFILE} unsafe-reset-all --priv_validator_secret '1' # 1>${BEATOZ_LOG} 2>&1
fi

echo "Start beatoz ..."
nohup ${BEATOZ_BINFILE} start --rpc.laddr 'tcp://0.0.0.0:26657' --rpc.cors_allowed_origins '*' --priv_validator_secret '1' 1>${BEATOZ_LOG} 2>&1 &