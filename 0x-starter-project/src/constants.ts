import { BigNumber } from '@0x/utils';

// tslint:disable-next-line:custom-no-magic-numbers
export const ONE_SECOND_MS = 1000;
// tslint:disable-next-line:custom-no-magic-numbers
export const ONE_MINUTE_MS = ONE_SECOND_MS * 60;
// tslint:disable-next-line:custom-no-magic-numbers
export const TEN_MINUTES_MS = ONE_MINUTE_MS * 10;
// tslint:disable-next-line:custom-no-magic-numbers
export const UNLIMITED_ALLOWANCE_IN_BASE_UNITS = new BigNumber(2).pow(256).minus(1);
// tslint:disable-next-line:custom-no-magic-numbers
export const DECIMALS = 18;
export const NULL_ADDRESS = '0x0000000000000000000000000000000000000000';
export const ETH_ADDRESS = '0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee';
export const NULL_BYTES = '0x';
export const ZERO = new BigNumber(0);
export const GANACHE_NETWORK_ID = 50;
export const KOVAN_NETWORK_ID = 42;
export const ROPSTEN_NETWORK_ID = 3;
export const RINKEBY_NETWORK_ID = 4;
export const MOCK_0x_API_BASE_URL = 'https://mock.api.0x.org';
