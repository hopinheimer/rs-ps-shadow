# Gossipsub simulator

this simulator tries to simulate a network and message pubsub using the gossipsub protocol. it's done using shadow and rustlibp2p but does one crazy stuff that it pushes all the events to grafana
but there's still lot of work to be done. my ambitious plan is to make the simulator modular giving us the option to switch between shadow and kubernetes

``` 
docker compose up
```
```
bash run_sim.sh
```
in a different tmux window(please use tmux if haven't already)

```
tail -qF ./shadow-1024.data/hosts/node*/rs-ps-shadow.1000.stdout | python3 parse.py
```

> :warning: if it isn't evident the code pretty jerryrigged please proceed with caution
