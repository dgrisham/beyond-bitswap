#!/bin/bash

for ((i=0; i < 1; ++i)); do
    dd if=/dev/urandom bs=1024 count=1000 of=./testDatasets/random-1MB
done
