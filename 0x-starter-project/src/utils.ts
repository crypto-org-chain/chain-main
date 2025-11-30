import { runMigrationsOnceAsync } from '@0x/migrations';
import { BigNumber } from '@0x/utils';
import { Web3Wrapper } from '@0x/web3-wrapper';
// tslint:disable-next-line:no-implicit-dependencies
import * as ethers from 'ethers';

import { GANACHE_CONFIGS, NETWORK_CONFIGS, TX_DEFAULTS } from './configs';
import { ONE_SECOND_MS, TEN_MINUTES_MS } from './constants';
import { providerEngine } from './provider_engine';

// HACK prevent ethers from printing 'Multiple definitions for'
ethers.errors.setLogLevel('error');

/**
 * Returns an amount of seconds that is greater than the amount of seconds since epoch.
 */
export const getRandomFutureDateInSeconds = (): BigNumber => {
    return new BigNumber(Date.now() + TEN_MINUTES_MS).div(ONE_SECOND_MS).integerValue(BigNumber.ROUND_CEIL);
};

export const runMigrationsOnceIfRequiredAsync = async (): Promise<void> => {
    if (NETWORK_CONFIGS === GANACHE_CONFIGS) {
        const web3Wrapper = new Web3Wrapper(providerEngine);
        const [owner] = await web3Wrapper.getAvailableAddressesAsync();
        await runMigrationsOnceAsync(providerEngine, { from: owner });
    }
};

export const calculateProtocolFee = (
    numOrders: number,
    multiplier: BigNumber,
    gasPrice: BigNumber | number = TX_DEFAULTS.gasPrice,
): BigNumber => {
    return multiplier.times(gasPrice).times(numOrders);
};
