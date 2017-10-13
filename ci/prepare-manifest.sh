#!/bin/bash

set -eu
set -x

cp build/manifest-template.yml result/manifest.yml
printf "\ndomain: $DOMAIN\n" >> result/manifest.yml
