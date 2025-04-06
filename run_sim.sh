#!/bin/bash

set -e

cargo build 

D=8
kb=1024
result=$((kb * 1024))
filename=shadow-$kb
interval=700
python3 network_graph.py 80 20 $result 1 $D $interval

shadow --progress true -d $filename.data shadow.yaml
#tar -czf $filename.tar.gz $filename.data

rm shadow.yaml
#rm -rf $filename.data

