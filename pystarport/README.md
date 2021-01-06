pystarport is like a [cosmos starport](https://github.com/tendermint/starport)
without the scaffolding feature. it's mainly used for development and testing. It's developed for cryptocom's chain, but
it can also be used for any cosmos-sdk based projects.

## Configuration

a typical configuration for devnet is like this:

```
chain_id: chainmaind
validators:
  - coins: 10cro
    staked: 10cro
  - coins: 10cro
    staked: 10cro
accounts:
  - name: community
    coins: 100cro
  - name: ecosystem
    coins: 200cro
  - name: reserve
    coins: 200cro
    vesting: "1d"
  - name: launch
    coins: 100cro
genesis:
 app_state:
   staking:
     params:
       unbonding_time: "10s"
```

The `validators` section defines how many nodes to run, for each node, a home directory is initialized in
`data/node{i}`, and a validator account with specified coins is created.

The `accounts` defines other non-validator accounts, they are created in `node0`'s keyring.

In the `genesis` section you can override any genesis configuration with the same json path.

## Usage

```
NAME
    pystarport serve - prepare and start a devnet from scatch

SYNOPSIS
    pystarport serve <flags>

DESCRIPTION
    prepare and start a devnet from scatch

FLAGS
    --data=DATA
        Type: str
        Default: './data'
        path to the root data directory
    --config=CONFIG
        Type: str
        Default: './config.yaml'
        path to the configuration file
    --base_port=BASE_PORT
        Type: int
        Default: 26650
        the base port to use, the service ports of different nodes are calculated based on this
    --cmd=CMD
        Type: str
        Default: 'chain-maind'
        the chain binary to use
```

## Port rules

The rules to calculate service ports based on base port is defined in the
[`ports.py`](https://github.com/crypto-com/chain-main/blob/master/pystarport/pystarport/ports.py) module.

For example, with default base port `26650`, the url of api servers of the nodes would be:

- Node0: http://127.0.0.1:26654
- Node1: http://127.0.0.1:26664

> The swagger doc of node0 is http://127.0.0.1:26654/swagger/
>
> The default rpc port used by `chain-maind` is `26657`, that's the default node0's rpc port, so you can use
> `chain-maind` without change to access node0's rpc.

## Supervisor

`pystarport` embeded a [supervisor](http://supervisord.org/) to manage processes of multiple nodes, you can use
`pystarport supervisorctl` to manage the processes:

```
$ pystarport supervisorctl status
node0                            RUNNING   pid 35210, uptime 0:00:29
node1                            RUNNING   pid 35211, uptime 0:00:29
$ pystarport supervisorctl help

default commands (type help <topic>):
=====================================
add    exit      open  reload  restart   start   tail
avail  fg        pid   remove  shutdown  status  update
clear  maintail  quit  reread  signal    stop    version
```

Or enter an interactive shell:

```
$ pystarport supervisorctl
node0                            RUNNING   pid 35210, uptime 0:01:53
node1                            RUNNING   pid 35211, uptime 0:01:53
supervisor>
```

## Cli

After started the chain, you can use `chain-maind` cli directly, there are also some wrapper commands provided by
`pystarport cli`. It understands the directory structure and port rules, also assuming `keyring-backend=test`, and there
are shortcuts for commonly used commands, so arguments are shorter.

```
$ pystarport cli - --help
...
```

## Transaction Bot

A simple transaction bot that works for cluster created by pystarport as well as a local node

Copy and modify `bot.yaml.sample` to `bot.yaml` with your desired bot configurations.

### If you are running on a pystarport created cluster:
1. Make sure you have provide the `node` for each job in the `bot.yaml`
2. Run the command
```
$ pystarport bot --chain-id=[cluster_chain_id] - start
```

### If you are running on a local node
```
$ pstarport bot --node_rpc=tcp://127.0.0.1:26657 --data=/path/to/your/local/node/home/ - start
```

## docker-compose

When used with `docker-compose` or multiple machines, you need to config hostnames, and you probabely want to use a same
`base_port` since you don't have port conflicts, you can config the `validators` like this:

```yaml
validators:
  - coins: 10cro
    staked: 10cro
    base_port: 26650
    hostname: node0
  - coins: 10cro
    staked: 10cro
    base_port: 26650
    hostname: node1
```

`pystarport init --gen_compose_file` will also generate a `docker-compose.yml` file for you.
