import { DummyERC721TokenContract } from '@0x/contracts-erc721';

import { GANACHE_CONFIGS, NETWORK_CONFIGS } from './configs';
import { ROPSTEN_NETWORK_ID } from './constants';
import { providerEngine } from './provider_engine';

const ERC721_TOKENS_BY_CHAIN_ID: { [chainId: number]: string[] } = {
    [GANACHE_CONFIGS.chainId]: ['0x07f96aa816c1f244cbc6ef114bb2b023ba54a2eb'],
    [ROPSTEN_NETWORK_ID]: [],
};

export const dummyERC721TokenContracts: DummyERC721TokenContract[] = [];

for (const tokenAddress of ERC721_TOKENS_BY_CHAIN_ID[NETWORK_CONFIGS.chainId]) {
    dummyERC721TokenContracts.push(new DummyERC721TokenContract(tokenAddress, providerEngine));
}
