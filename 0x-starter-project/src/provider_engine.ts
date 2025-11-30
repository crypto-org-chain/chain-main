import { GanacheSubprovider, MnemonicWalletSubprovider, RPCSubprovider, Web3ProviderEngine } from '@0x/subproviders';
import { providerUtils } from '@0x/utils';

import { BASE_DERIVATION_PATH, GANACHE_CONFIGS, MNEMONIC, NETWORK_CONFIGS } from './configs';

export const mnemonicWallet = new MnemonicWalletSubprovider({
    mnemonic: MNEMONIC,
    baseDerivationPath: BASE_DERIVATION_PATH,
    chainId: NETWORK_CONFIGS.chainId,
});

const determineProvider = (): Web3ProviderEngine => {
    const pe = new Web3ProviderEngine();
    pe.addProvider(mnemonicWallet);
    if (NETWORK_CONFIGS === GANACHE_CONFIGS) {
        pe.addProvider(
            new GanacheSubprovider({
                vmErrorsOnRPCResponse: false,
                network_id: GANACHE_CONFIGS.networkId,
                _chainId: GANACHE_CONFIGS.chainId,
                mnemonic: MNEMONIC,
            }),
        );
    } else {
        pe.addProvider(new RPCSubprovider(NETWORK_CONFIGS.rpcUrl));
    }
    providerUtils.startProviderEngine(pe);
    return pe;
};

export const providerEngine = determineProvider();
