import { ContractWrappers, ERC20TokenContract } from '@0x/contract-wrappers';
import { OrderStatus, OtcOrder, SignatureType } from '@0x/protocol-utils';
import { BigNumber } from '@0x/utils';
import { Web3Wrapper } from '@0x/web3-wrapper';

import { NETWORK_CONFIGS, TX_DEFAULTS } from '../configs';
import { DECIMALS, UNLIMITED_ALLOWANCE_IN_BASE_UNITS } from '../constants';
import { PrintUtils } from '../print_utils';
import { providerEngine } from '../provider_engine';
import { getRandomFutureDateInSeconds, runMigrationsOnceIfRequiredAsync } from '../utils';

/**
 * In this scenario, the maker creates and signs an OTC order for selling
 * ZRX for WETH.
 *
 * The taker takes this order and fills it via the 0x Exchange Proxy contract.
 */
export async function scenarioAsync(): Promise<void> {
    await runMigrationsOnceIfRequiredAsync();
    PrintUtils.printScenario('Fill ERC20 OTC Order');
    // Initialize the ContractWrappers, this provides helper functions around calling
    // 0x contracts as well as ERC20/ERC721 token contracts on the blockchain
    const contractWrappers = new ContractWrappers(providerEngine, { chainId: NETWORK_CONFIGS.chainId });
    // Initialize the Web3Wrapper, this provides helper functions around fetching
    // account information, balances, general contract logs
    const web3Wrapper = new Web3Wrapper(providerEngine);
    const [maker, taker] = await web3Wrapper.getAvailableAddressesAsync();
    const zrxTokenAddress = contractWrappers.contractAddresses.zrxToken;
    const etherTokenAddress = contractWrappers.contractAddresses.etherToken;
    const printUtils = new PrintUtils(
        web3Wrapper,
        contractWrappers,
        { maker, taker },
        { WETH: etherTokenAddress, ZRX: zrxTokenAddress },
    );
    printUtils.printAccounts();

    // the amount the maker is selling of maker asset
    const makerAssetAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(5), DECIMALS);
    // the amount the maker wants of taker asset
    const takerAssetAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(0.1), DECIMALS);

    // Allow the 0x Exchange Proxy to move ZRX on behalf of the maker
    const erc20Token = new ERC20TokenContract(zrxTokenAddress, providerEngine);
    const makerZRXApprovalTxHash = await erc20Token
        .approve(contractWrappers.contractAddresses.exchangeProxy, UNLIMITED_ALLOWANCE_IN_BASE_UNITS)
        .sendTransactionAsync({ from: maker, ...TX_DEFAULTS });
    await printUtils.awaitTransactionMinedSpinnerAsync('Maker ZRX Approval', makerZRXApprovalTxHash);

    // Allow the 0x Exchange Proxy to move WETH on behalf of the taker
    const etherToken = contractWrappers.weth9;
    const takerWETHApprovalTxHash = await etherToken
        .approve(contractWrappers.contractAddresses.exchangeProxy, UNLIMITED_ALLOWANCE_IN_BASE_UNITS)
        .sendTransactionAsync({ from: taker, ...TX_DEFAULTS });
    await printUtils.awaitTransactionMinedSpinnerAsync('Taker WETH Approval', takerWETHApprovalTxHash);

    // Convert ETH into WETH for taker by depositing ETH into the WETH contract
    const takerWETHDepositTxHash = await etherToken.deposit().sendTransactionAsync({
        from: taker,
        value: takerAssetAmount,
        ...TX_DEFAULTS,
    });
    await printUtils.awaitTransactionMinedSpinnerAsync('Taker WETH Deposit', takerWETHDepositTxHash);

    PrintUtils.printData('Setup', [
        ['Maker ZRX Approval', makerZRXApprovalTxHash],
        ['Taker WETH Approval', takerWETHApprovalTxHash],
        ['Taker WETH Deposit', takerWETHDepositTxHash],
    ]);

    // Set up the Order and fill it
    const randomExpiration = getRandomFutureDateInSeconds();
    const nonceBucket = new BigNumber(0);
    const nonce = (await contractWrappers.exchangeProxy
        .lastOtcTxOriginNonce(taker, nonceBucket)
        .callAsync()).plus(1);

    const expiryAndNonce = OtcOrder.encodeExpiryAndNonce(randomExpiration, nonceBucket, nonce);

    // Create the order
    const otcOrder: OtcOrder = new OtcOrder({
        chainId: NETWORK_CONFIGS.chainId,
        verifyingContract: contractWrappers.contractAddresses.exchangeProxy,
        maker,
        taker,
        makerToken: zrxTokenAddress,
        takerToken: etherTokenAddress,
        makerAmount: makerAssetAmount,
        takerAmount: takerAssetAmount,
        txOrigin: taker,
        expiryAndNonce,
    });

    // Print order
    printUtils.printOrder(otcOrder);

    // Print out the Balances and Allowances
    await printUtils.fetchAndPrintContractAllowancesAsync(contractWrappers.contractAddresses.exchangeProxy);
    await printUtils.fetchAndPrintContractBalancesAsync();

    // Generate the order hash and sign it
    const signature = await otcOrder.getSignatureWithProviderAsync(web3Wrapper.getProvider(), SignatureType.EthSign, maker);

    const {
        orderHash,
        status,
    } = await contractWrappers.exchangeProxy.getOtcOrderInfo(otcOrder).callAsync();
    if (status === OrderStatus.Fillable) {
        // Order is fillable
    }

    // Fill the Order via 0x Exchange Proxy contract
    const txHash = await contractWrappers.exchangeProxy
        .fillOtcOrder(otcOrder, signature, takerAssetAmount)
        .sendTransactionAsync({
            from: taker,
            ...TX_DEFAULTS,
        });
    const txReceipt = await printUtils.awaitTransactionMinedSpinnerAsync('fillOtcOrder', txHash);
    printUtils.printTransaction('fillOtcOrder', txReceipt, [['orderHash', orderHash]]);

    // Print the Balances
    await printUtils.fetchAndPrintContractBalancesAsync();

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
