#!/bin/bash

for ((i=0; i < 100; ++i)); do
    dd if=/dev/urandom bs=1024 count=10000 of=./test-datasets/random-1GB$i
done
