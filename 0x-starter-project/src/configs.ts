import * as dotenv from 'dotenv';
dotenv.config();

import { GANACHE_NETWORK_ID, ROPSTEN_NETWORK_ID } from './constants';
import { NetworkSpecificConfigs } from './types';

export const TX_DEFAULTS = { gas: 800000, gasPrice: 20e9 };
export const MNEMONIC = 'concert load couple harbor equip island argue ramp clarify fence smart topic';
export const BASE_DERIVATION_PATH = `44'/60'/0'/0`;
export const GANACHE_CONFIGS: NetworkSpecificConfigs = {
    rpcUrl: 'http://127.0.0.1:8545',
    networkId: GANACHE_NETWORK_ID,
    chainId: 1337,
};
export const ROPSTEN_CONFIGS: NetworkSpecificConfigs = {
    rpcUrl: process.env.ROPSTEN_RPC_URL || '',
    networkId: ROPSTEN_NETWORK_ID,
    chainId: ROPSTEN_NETWORK_ID,
};

export const NETWORK_CONFIGS = GANACHE_CONFIGS; // or ROPSTEN_CONFIGS
