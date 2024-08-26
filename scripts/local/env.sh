#!/usr/bin/env bash

HOSTOS="linux"
if [[ "$OSTYPE" == "darwin"* ]]; then
  HOSTOS="darwin"
fi
echo "Your OS is $HOSTOS"

EXE="$GOPATH/src/github.com/beatoz/beatoz-go/build/${HOSTOS}/beatoz"

echo ${1}
NODE_HOMES=()
IDX=0
while [ $IDX -lt ${1} ]
do
  NODE_HOMES[$IDX]="$HOME/beatoz_localnet_$IDX"
  echo ${NODE_HOMES[$IDX]}
  IDX=$(($IDX + 1))
done
