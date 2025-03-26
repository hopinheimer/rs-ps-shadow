#!/bin/bash

set -e

cargo build 

D=8

for kb in 128 256 512 1024 2048 4096 8192; do
      result=$((kb * 1024))
      filename=shadow-$kb
      interval=1500
      python3 network_graph.py 10 10 $result 1 $D $interval

      shadow --progress true -d $filename.data shadow.yaml

      tar -czf $filename.tar.gz $filename.data

      rm shadow.yaml
      rm -rf $filename.data
done

for num_msgs in 2 4 8 16 32 64; do
    result=$((128 * 1024))
    filename=shadow-128-$num_msgs
    interval=1500
    python3 network_graph.py 10 10 $result $num_msgs $D $interval

    shadow --progress true -d $filename.data shadow.yaml

    tar -czf $filename.tar.gz $filename.data

    rm shadow.yaml
    rm -rf $filename.data
done
