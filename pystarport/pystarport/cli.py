import re
import os
import json
from pathlib import Path
import asyncio
import fire
import tomlkit
from .utils import execute, interact, local_ip
from .ports import p2p_port, rpc_port, api_port, grpc_port, pprof_port

CHAIN = 'chain-maind'
CHAIN_ID = 'chainmaind'
BASE_PORT = 26600


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


async def init(count):
    await interact('rm -r data', ignore_error=True)
    for i in range(count):
        await chaind('init', f'node{i}', chain_id=CHAIN_ID, home=f'data/node{i}')
    await interact('mv data/node0/config/genesis.json data/')
    await interact('mkdir data/gentx')
    for i in range(count):
        await interact(f'ln -sf ../../genesis.json data/node{i}/config/')
        await interact(f'ln -sf ../../gentx data/node{i}/config/')

    # create accounts
    for i in range(count):
        output = await chaind('keys', 'add', 'validator', home=f'data/node{i}', output='json', keyring_backend='test')
        account = json.loads(output)
        print('account', account)
        await chaind('add-genesis-account', account['address'], '1cro', home=f'data/node{i}')
        await chaind('gentx', 'validator', amount='1cro', keyring_backend='test', chain_id=CHAIN_ID, home=f'data/node{i}')

    # collect-gentxs
    await chaind('collect-gentxs', gentx_dir='data/gentx', home=f'data/node0')

    # write tendermint config
    ip = local_ip()
    peers = ','.join(['tcp://%s@%s:%d' % (await node_id(i), ip, p2p_port(BASE_PORT, i)) for i in range(count)])
    for i in range(count):
        edit_tm_cfg(f'data/node{i}/config/config.toml', BASE_PORT, i, peers)
        edit_app_cfg(f'data/node{i}/config/app.toml', BASE_PORT, i)


async def start():
    count = len([name for name in os.listdir('data') if re.match(r'^node\d+$', name)])
    if count == 0:
        print('not initialized yet', file=sys.stderr)
        return
    for i in range(count):
        Path(f'data/node{i}.log').touch()
    tasks = [execute('tail -f ' + ' '.join(f'data/node{i}.log' for i in range(count)))] + \
        [execute(f'{CHAIN} start --home data/node{i} > data/node{i}.log') for i in range(count)]
    await asyncio.wait(tasks, return_when=asyncio.FIRST_COMPLETED)


async def serve(count):
    await execute('go install -mod readonly ./cmd/chain-maind')
    await init(count)
    await start()


class CLI:
    def init(self, count=1):
        '''
        initialize testnet data directory
        '''
        asyncio.run(init(count))

    def start(self):
        '''
        start testnet processes
        '''
        asyncio.run(start())

    def serve(self, count=1):
        '''
        build, init and start
        '''
        asyncio.run(serve(count))


def main():
    fire.Fire(CLI())

if __name__ == '__main__':
    main()
