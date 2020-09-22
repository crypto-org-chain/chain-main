import re
import os
import sys
import signal
import datetime
import json
from pathlib import Path
import asyncio
import atexit

import fire
import tomlkit
import yaml
import jsonmerge
import dateutil.parser
import durations

from .utils import interact, local_ip
from .ports import p2p_port, rpc_port, api_port, grpc_port, pprof_port

CHAIN = 'chain-maind'  # edit by nix-build
CHAIN_ID = 'chainmaind'
BASE_PORT = 26650


async def node_id(i):
    output = await chaind('tendermint', 'show-node-id', home=f'data/node{i}')
    return output.decode().strip()


async def chaind(*args, **kwargs):
    args = list(args)
    for k,v in kwargs.items():
        args.append('--' + k.replace('_', '-'))
        args.append(v)
    return await interact(' '.join((CHAIN, *args)))


def edit_tm_cfg(path, base_port, i, peers):
    doc = tomlkit.parse(open(path).read())
    doc['moniker'] = 'node%d' % i
    # tendermint is start in process, not needed
    # doc['proxy_app'] = 'tcp://127.0.0.1:%d' % abci_port(base_port, i)
    doc['rpc']['laddr'] = 'tcp://0.0.0.0:%d' % rpc_port(base_port, i)
    doc['rpc']['pprof_laddr'] = 'localhost:%d' % pprof_port(base_port, i)
    doc['p2p']['laddr'] = 'tcp://0.0.0.0:%d' % p2p_port(base_port, i)
    doc['p2p']['persistent_peers'] = peers
    doc['p2p']['addr_book_strict'] = False
    doc['p2p']['allow_duplicate_ip'] = True
    open(path, 'w').write(tomlkit.dumps(doc))


def edit_app_cfg(path, base_port, i):
    doc = tomlkit.parse(open(path).read())
    doc['api']['address'] = "tcp://0.0.0.0:%d" % api_port(base_port, i)
    doc['grpc']['address'] = "0.0.0.0:%d" % grpc_port(base_port, i)
    open(path, 'w').write(tomlkit.dumps(doc))


async def create_account(i, name):
    output = await chaind('keys', 'add', name, home=f'data/node{i}', output='json', keyring_backend='test')
    return json.loads(output)


async def init(config):
    await interact('rm -r data', ignore_error=True)
    for i in range(len(config['validators'])):
        await chaind('init', f'node{i}', chain_id=CHAIN_ID, home=f'data/node{i}')
    await interact('mv data/node0/config/genesis.json data/')
    await interact('mkdir data/gentx')
    for i in range(len(config['validators'])):
        await interact(f'ln -sf ../../genesis.json data/node{i}/config/')
        await interact(f'ln -sf ../../gentx data/node{i}/config/')

    # patch the genesis file
    genesis = jsonmerge.merge(json.load(open('data/genesis.json')), config.get('genesis', {}))
    json.dump(genesis, open('data/genesis.json', 'w'))
    await chaind('validate-genesis', home=f'data/node0')

    # create accounts
    for i, node in enumerate(config['validators']):
        account = await create_account(i, 'validator')
        print(account)
        await chaind('add-genesis-account', account['address'], node['coins'], home=f'data/node{i}')
        await chaind('gentx', 'validator', amount=node['staked'], keyring_backend='test', chain_id=CHAIN_ID, home=f'data/node{i}')

    for account in config['accounts']:
        acct = await create_account(0, account['name'])
        print(acct)
        vesting = account.get('vesting')
        if not vesting:
            await chaind('add-genesis-account', acct['address'], account['coins'], home=f'data/node0')
        else:
            genesis_time = dateutil.parser.isoparse(genesis['genesis_time'])
            end_time = genesis_time + datetime.timedelta(seconds=durations.Duration(vesting).to_seconds())
            await chaind('add-genesis-account', acct['address'], account['coins'], home=f'data/node0',
                         vesting_amount=account['coins'],
                         vesting_end_time=end_time.replace(tzinfo=None).isoformat('T')+'Z')

    # collect-gentxs
    await chaind('collect-gentxs', gentx_dir='data/gentx', home=f'data/node0')

    # write tendermint config
    ip = local_ip()
    peers = ','.join([
        'tcp://%s@%s:%d' % (await node_id(i), ip, p2p_port(BASE_PORT, i))
        for i in range(len(config['validators']))
    ])
    for i in range(len(config['validators'])):
        edit_tm_cfg(f'data/node{i}/config/config.toml', BASE_PORT, i, peers)
        edit_app_cfg(f'data/node{i}/config/app.toml', BASE_PORT, i)


async def start(quiet):
    count = len([name for name in os.listdir('data') if re.match(r'^node\d+$', name)])
    if count == 0:
        print('not initialized yet', file=sys.stderr)
        return
    for i in range(count):
        Path(f'data/node{i}.log').touch()
    processes = [await asyncio.create_subprocess_shell(f'{CHAIN} start --home data/node{i} > data/node{i}.log', preexec_fn=os.setsid) for i in range(count)]
    if not quiet:
        processes.append(await asyncio.create_subprocess_exec('tail', '-f', *(f'data/node{i}.log' for i in
                                                                              range(count))))

    def terminate():
        print('terminate child processes')
        for p in processes:
            os.killpg(os.getpgid(p.pid), signal.SIGTERM)
    atexit.register(terminate)

    await asyncio.wait([p.wait() for p in processes], return_when=asyncio.FIRST_COMPLETED)


async def serve(config, quiet):
    await init(config)
    await start(quiet)


class CLI:
    def init(self, config):
        '''
        initialize testnet data directory
        '''
        asyncio.run(init(yaml.safe_load(open(config))))

    def start(self, tail=True):
        '''
        start testnet processes
        '''
        asyncio.run(start(tail))

    def serve(self, config, quiet=False):
        '''
        build, init and start
        '''
        asyncio.run(serve(yaml.safe_load(open(config)), quiet))


def terminate(*args):
    sys.exit(1)


def main():
    signal.signal(signal.SIGTERM, terminate)
    fire.Fire(CLI())

if __name__ == '__main__':
    main()
