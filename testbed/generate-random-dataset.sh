#!/bin/bash

for ((i=0; i < 2; ++i)); do
    dd if=/dev/urandom bs=1024 count=1000000 of=./testDatasets/random-1GB-$i
done
