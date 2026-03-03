import { ContractWrappers } from '@0x/contract-wrappers';
// tslint:disable-next-line:no-implicit-dependencies
import { DummyERC20TokenContract } from '@0x/contracts-erc20';
import { BigNumber } from '@0x/utils';
import { Web3Wrapper } from '@0x/web3-wrapper';

import { NETWORK_CONFIGS, TX_DEFAULTS } from '../configs';
import { DECIMALS } from '../constants';
import { PrintUtils } from '../print_utils';
import { providerEngine } from '../provider_engine';

async function mainAsync(): Promise<void> {
    const contractWrappers = new ContractWrappers(providerEngine, { chainId: NETWORK_CONFIGS.chainId });

    const web3Wrapper = new Web3Wrapper(providerEngine);

    const [maker] = await web3Wrapper.getAvailableAddressesAsync();
    const zrxTokenAddress = contractWrappers.contractAddresses.zrxToken;
    const zrxToken = new DummyERC20TokenContract(zrxTokenAddress, providerEngine);

    const printUtils = new PrintUtils(web3Wrapper, contractWrappers, { maker }, { ZRX: zrxTokenAddress });

    await printUtils.fetchAndPrintContractBalancesAsync();

    const mintAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(10000), DECIMALS);

    const mintZrxTx = await zrxToken.mint(mintAmount).sendTransactionAsync({ from: maker, ...TX_DEFAULTS });
    await printUtils.awaitTransactionMinedSpinnerAsync('Mint Test ZRX for Maker', mintZrxTx);

    await printUtils.fetchAndPrintContractBalancesAsync();
}

mainAsync().catch((error: any) => {
    console.error(error);
    process.exitCode = 1;
});
