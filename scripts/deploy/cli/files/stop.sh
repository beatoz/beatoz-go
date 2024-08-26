#!/usr/bin/env bash

kill -15 `ps -ef | grep beatoz | grep -v 'grep' | awk '{print $2}'`