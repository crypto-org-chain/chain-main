import { ContractWrappers } from '@0x/contract-wrappers';
import { LimitOrder, OrderInfo } from '@0x/protocol-utils';
import { BigNumber, hexUtils } from '@0x/utils';
import { Web3Wrapper } from '@0x/web3-wrapper';

import { NETWORK_CONFIGS, TX_DEFAULTS } from '../configs';
import { NULL_ADDRESS, ZERO } from '../constants';
import { PrintUtils } from '../print_utils';
import { providerEngine } from '../provider_engine';
import { getRandomFutureDateInSeconds, runMigrationsOnceIfRequiredAsync } from '../utils';

/**
 * In this scenario, the maker creates and signs many orders selling ZRX for WETH
 * and selling WETH for ZRX. The maker is able to cancel several orders at once
 * for a given maker token, taker token combination using cancelPairLimitOrders
 */
export async function scenarioAsync(): Promise<void> {
    await runMigrationsOnceIfRequiredAsync();
    PrintUtils.printScenario('cancelPairLimitOrders');
    // Initialize the ContractWrappers, this provides helper functions around calling
    // 0x contracts as well as ERC20/ERC721 token contracts on the blockchain
    const contractWrappers = new ContractWrappers(providerEngine, { chainId: NETWORK_CONFIGS.chainId });
    // Initialize the Web3Wrapper, this provides helper functions around fetching
    // account information, balances, general contract logs
    const web3Wrapper = new Web3Wrapper(providerEngine);
    const [maker, taker] = await web3Wrapper.getAvailableAddressesAsync();
    const exchangeProxyAddress = contractWrappers.contractAddresses.exchangeProxy;
    const zrxTokenAddress = contractWrappers.contractAddresses.zrxToken;
    const etherTokenAddress = contractWrappers.contractAddresses.etherToken;
    const printUtils = new PrintUtils(
        web3Wrapper,
        contractWrappers,
        { maker, taker },
        { WETH: etherTokenAddress, ZRX: zrxTokenAddress },
    );
    printUtils.printAccounts();

    // the amount of ZRX involved in the trade
    const zrxAmount = new BigNumber(100);
    // the amount of WETH involved in the trade
    const wethAmount = new BigNumber(10);

    const randomExpiration = getRandomFutureDateInSeconds();
    const pool = hexUtils.leftPad(1);

    // Rather than using a random salt, we use an incrementing salt value.
    // First we'll create some orders selling ZRX for WETH.
    const sellZrxLimitOrder1: LimitOrder = new LimitOrder({
        chainId: NETWORK_CONFIGS.chainId,
        verifyingContract: exchangeProxyAddress,
        maker,
        taker,
        makerToken: zrxTokenAddress,
        takerToken: etherTokenAddress,
        makerAmount: zrxAmount,
        takerAmount: wethAmount,
        takerTokenFeeAmount: ZERO,
        sender: NULL_ADDRESS,
        feeRecipient: NULL_ADDRESS,
        expiry: randomExpiration,
        pool,
        salt: new BigNumber(1),
    });

    const sellZrxLimitOrder2: LimitOrder = new LimitOrder({
        ...sellZrxLimitOrder1,
        salt: new BigNumber(2),
    });

    const sellZrxLimitOrder3: LimitOrder = new LimitOrder({
        ...sellZrxLimitOrder1,
        salt: new BigNumber(3),
    });

    // Next we'll create some orders selling WETH for ZRX
    const sellWethLimitOrder1: LimitOrder = new LimitOrder({
        ...sellZrxLimitOrder1,
        makerToken: etherTokenAddress,
        takerToken: zrxTokenAddress,
        makerAmount: wethAmount,
        takerAmount: zrxAmount,
        salt: new BigNumber(1),
    });

    const sellWethLimitOrder2: LimitOrder = new LimitOrder({
        ...sellWethLimitOrder1,
        salt: new BigNumber(2),
    });

    const sellWethLimitOrder3: LimitOrder = new LimitOrder({
        ...sellWethLimitOrder1,
        salt: new BigNumber(3),
    });

    async function getLimitOrderInfoAsync(limitOrder: LimitOrder): Promise<OrderInfo> {
        return contractWrappers.exchangeProxy.getLimitOrderInfo(limitOrder).callAsync();
    }
    async function fetchAndPrintLimitOrderInfosAsync(): Promise<void> {
        // Fetch and print the order info
        const sellZrxOrder1Info = await getLimitOrderInfoAsync(sellZrxLimitOrder1);
        const sellZrxOrder2Info = await getLimitOrderInfoAsync(sellZrxLimitOrder2);
        const sellZrxOrder3Info = await getLimitOrderInfoAsync(sellZrxLimitOrder3);
        const sellWethOrder1Info = await getLimitOrderInfoAsync(sellWethLimitOrder1);
        const sellWethOrder2Info = await getLimitOrderInfoAsync(sellWethLimitOrder2);
        const sellWethOrder3Info = await getLimitOrderInfoAsync(sellWethLimitOrder3);
        printUtils.printOrderInfos({
            sellZrxOrder1: sellZrxOrder1Info,
            sellZrxOrder2: sellZrxOrder2Info,
            sellZrxOrder3: sellZrxOrder3Info,
            sellWethOrder1: sellWethOrder1Info,
            sellWethOrder2: sellWethOrder2Info,
            sellWethOrder3: sellWethOrder3Info,
        });
    }

    // Currently all orders should be fillable
    await fetchAndPrintLimitOrderInfosAsync();

    // Maker cancels all ZRX -> WETH orders before and including order2.
    // ZRX -> WETH order3 remains valid.
    // All WETH -> ZRX orders remain valid.
    const minValidZrxWethSalt = sellZrxLimitOrder2.salt.plus(1);
    const cancelZrxWethOrdersTx = await contractWrappers.exchangeProxy.cancelPairLimitOrders(zrxTokenAddress, etherTokenAddress, minValidZrxWethSalt).sendTransactionAsync({
        from: maker,
        ...TX_DEFAULTS,
    });
    const cancelZrxWethOrdersReceipt = await printUtils.awaitTransactionMinedSpinnerAsync('cancelPairLimitOrders', cancelZrxWethOrdersTx);
    printUtils.printTransaction('cancelPairLimitOrder: ZRX-->WETH', cancelZrxWethOrdersReceipt, [
        ['targetSalt', minValidZrxWethSalt.toString()],
        ['makerToken', zrxTokenAddress],
        ['takerToken', etherTokenAddress],
    ]);
    // Now ZRX -> WETH orders 1 and 2 should be cancelled.
    // ZRX -> WETH order 3 as well as all WETH -> ZRX orders are unaffected.
    await fetchAndPrintLimitOrderInfosAsync();

    // Maker cancels all WETH -> ZRX orders before and including order2.
    // WETH -> ZRX order3 remains valid.
    const minValidWethZrxSalt = sellWethLimitOrder2.salt.plus(1);
    const cancelWethZrxOrdersTx = await contractWrappers.exchangeProxy.cancelPairLimitOrders(etherTokenAddress, zrxTokenAddress, minValidWethZrxSalt).sendTransactionAsync({
        from: maker,
        ...TX_DEFAULTS,
    });
    const cancelWethZrxOrdersReceipt = await printUtils.awaitTransactionMinedSpinnerAsync('cancelPairLimitOrders', cancelWethZrxOrdersTx);
    printUtils.printTransaction('cancelPairLimitOrder: WETH-->ZRX', cancelWethZrxOrdersReceipt, [
        ['minValidSalt', minValidWethZrxSalt.toString()],
        ['makerToken', etherTokenAddress],
        ['takerToken', zrxTokenAddress],
    ]);
    // Now additionally WETH -> ZRX orders 1 and 2 should be cancelled.
    await fetchAndPrintLimitOrderInfosAsync();

    // Stop the Provider Engine
    providerEngine.stop();
}

void (async () => {
    try {
        if (!module.parent) {
            await scenarioAsync();
        }
    } catch (e) {
        console.log(e);
        providerEngine.stop();
        process.exit(1);
    }
})();
