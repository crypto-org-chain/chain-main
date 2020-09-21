import re
import os
import json
from pathlib import Path
import asyncio
import atexit

import fire
import tomlkit
import yaml
import jsonmerge

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
        print('account', account)
        await chaind('add-genesis-account', account['address'], node['coins'], home=f'data/node{i}')
        await chaind('gentx', 'validator', amount=node['staked'], keyring_backend='test', chain_id=CHAIN_ID, home=f'data/node{i}')

    for account in config['accounts']:
        acct = await create_account(0, account['name'])
        await chaind('add-genesis-account', acct['address'], account['coins'], home=f'data/node0')

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


async def start():
    count = len([name for name in os.listdir('data') if re.match(r'^node\d+$', name)])
    if count == 0:
        print('not initialized yet', file=sys.stderr)
        return
    for i in range(count):
        Path(f'data/node{i}.log').touch()
    tail_process = await asyncio.create_subprocess_exec('tail', '-f', *(f'data/node{i}.log' for i in range(count)))
    chain_processes = [await asyncio.create_subprocess_exec(CHAIN, 'start', '--home', f'data/node{i}') for i in range(count)]
    def terminate():
        processes = chain_processes + [tail_process]
        for p in processes:
            try:
                p.terminate()
            except BaseException as e:
                print(e, file=sys.stderr)
    atexit.register(terminate)
    await asyncio.wait([tail_process.wait()] + [p.wait() for p in chain_processes], return_when=asyncio.FIRST_COMPLETED)


async def serve(config):
    await init(config)
    await start()


class CLI:
    def init(self, config):
        '''
        initialize testnet data directory
        '''
        asyncio.run(init(yaml.safe_load(open(config))))

    def start(self):
        '''
        start testnet processes
        '''
        asyncio.run(start())

    def serve(self, config):
        '''
        build, init and start
        '''
        asyncio.run(serve(yaml.safe_load(open(config))))


def main():
    fire.Fire(CLI())

if __name__ == '__main__':
    main()
