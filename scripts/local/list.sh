#!/usr/bin/env bash

# list only beatoz processes which is launched by start.sh
ps -e | grep 'beatoz start --home' | grep -v 'grep'