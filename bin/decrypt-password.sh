#!/bin/bash
set -e

if [[ $# < 2 ]]; then
    echo "Usage: RDSBROKER_ENCRYPTION_KEY=<key> $0 <encrypted_password> <iv>"
    echo "All arguments should be in hex"
    exit 1
fi

ENC_PASSWORD=$1
IV=$2

echo -n $ENC_PASSWORD | xxd -r -p | openssl enc -d -aes-256-cfb -K $RDSBROKER_ENCRYPTION_KEY -iv $IV
