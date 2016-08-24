# go-network-inerface-check

Description:

App checks intefaces statuses using /sys/class/net

Flags:

`-ignoreif` - list of interfaces, separted by comma,  states of which will be ignored (example: `-ignoreif=lo,eth1`), default = lo

`-debug` - show all inforamation (example: `-debug=true`), default - false

`-ubuntu` - read list of interfaces from ubuntu's /etc/network/interfaces file and it's auto field('s) (example: `-ubuntu=true`)

Full example:
```
go-network-inerface-check -ubuntu=true -ignoreif=eth0,eth1,eth2 -debug=true
```
