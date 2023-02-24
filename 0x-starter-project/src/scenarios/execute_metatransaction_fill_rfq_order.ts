import { ContractWrappers, ERC20TokenContract } from '@0x/contract-wrappers';
import { MetaTransaction, MetaTransactionFields, OrderStatus, RfqOrder, SignatureType } from '@0x/protocol-utils';
import { BigNumber, hexUtils, NULL_ADDRESS } from '@0x/utils';
import { Web3Wrapper } from '@0x/web3-wrapper';

import { NETWORK_CONFIGS, TX_DEFAULTS } from '../configs';
import { DECIMALS, UNLIMITED_ALLOWANCE_IN_BASE_UNITS } from '../constants';
import { PrintUtils } from '../print_utils';
import { providerEngine } from '../provider_engine';
import { getRandomFutureDateInSeconds, runMigrationsOnceIfRequiredAsync } from '../utils';

/**
 * In this scenario, we use the metatransaction functionality to submit
 * a fillRfqOrder call on behalf of a taker. This allows a third party
 * (we'll call them the sender) to pay gas and handle transaction
 * submission on behalf of a trader.
 *
 * Note that using a takerSignedOtcOrder will be more gas efficient
 * if it fits your needs.
 */
export async function scenarioAsync(): Promise<void> {
    await runMigrationsOnceIfRequiredAsync();
    PrintUtils.printScenario('Execute metatransaction with a fillRfqOrder call');
    // Initialize the ContractWrappers, this provides helper functions around calling
    // 0x contracts as well as ERC20/ERC721 token contracts on the blockchain.
    const contractWrappers = new ContractWrappers(providerEngine, { chainId: NETWORK_CONFIGS.chainId });
    // Initialize the Web3Wrapper, this provides helper functions around fetching
    // account information, balances, general contract logs.
    const web3Wrapper = new Web3Wrapper(providerEngine);
    // Note that anyone can submit a Taker-Signed Order on-chain.
    // We fetch usable addresses for the parties to the trade (maker and taker)
    // as well as a separate address that actually sends the transaction.
    const [maker, taker, sender] = await web3Wrapper.getAvailableAddressesAsync();
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

    // the amount the maker is selling of maker token
    const makerTokenAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(5), DECIMALS);
    // the amount the maker wants of taker token
    const takerTokenAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(0.1), DECIMALS);
    let txHash;
    let txReceipt;

    // Allow the 0x Exchange Proxy to move ZRX on behalf of makerAccount
    const erc20Token = new ERC20TokenContract(zrxTokenAddress, providerEngine);
    const makerZRXApprovalTxHash = await erc20Token
        .approve(exchangeProxyAddress, UNLIMITED_ALLOWANCE_IN_BASE_UNITS)
        .sendTransactionAsync({ from: maker, ...TX_DEFAULTS });
    await printUtils.awaitTransactionMinedSpinnerAsync('Maker ZRX Approval', makerZRXApprovalTxHash);

    // Allow the 0x Exchange Proxy to move WETH on behalf of takerAccount
    const etherToken = contractWrappers.weth9;
    const takerWETHApprovalTxHash = await etherToken
        .approve(exchangeProxyAddress, UNLIMITED_ALLOWANCE_IN_BASE_UNITS)
        .sendTransactionAsync({ from: taker, ...TX_DEFAULTS });
    await printUtils.awaitTransactionMinedSpinnerAsync('Taker WETH Approval', takerWETHApprovalTxHash);

    // Convert ETH into WETH for taker by depositing ETH into the WETH contract
    const takerWETHDepositTxHash = await etherToken.deposit().sendTransactionAsync({
        from: taker,
        value: takerTokenAmount,
    });
    await printUtils.awaitTransactionMinedSpinnerAsync('Taker WETH Deposit', takerWETHDepositTxHash);

    PrintUtils.printData('Setup', [
        ['Maker ZRX Approval', makerZRXApprovalTxHash],
        ['Taker WETH Approval', takerWETHApprovalTxHash],
        ['Taker WETH Deposit', takerWETHDepositTxHash],
    ]);

    // Set up the Order and fill it
    const randomExpiration = getRandomFutureDateInSeconds();
    const pool = hexUtils.leftPad(1);

    // Create the order
    const rfqOrder: RfqOrder = new RfqOrder({
        chainId: NETWORK_CONFIGS.chainId,
        verifyingContract: exchangeProxyAddress,
        maker,
        taker,
        makerToken: zrxTokenAddress,
        takerToken: etherTokenAddress,
        makerAmount: makerTokenAmount,
        takerAmount: takerTokenAmount,
        // This needs to be the sender of the meta-transaction.
        txOrigin: sender,
        expiry: randomExpiration,
        pool,
        salt: new BigNumber(Date.now()),
    });

    // Print order
    printUtils.printOrder(rfqOrder);

    // Print out the Balances and Allowances
    await printUtils.fetchAndPrintContractAllowancesAsync(contractWrappers.contractAddresses.exchangeProxy);
    await printUtils.fetchAndPrintContractBalancesAsync();

    // Have the maker sign the order
    const makerSignature = await rfqOrder.getSignatureWithProviderAsync(web3Wrapper.getProvider(), SignatureType.EthSign, maker);

    const [
        { orderHash, status },
        remainingFillableAmount,
        isValidSignature,
    ] = await contractWrappers.exchangeProxy.getRfqOrderRelevantState(rfqOrder, makerSignature).callAsync();
    if (status === OrderStatus.Fillable && remainingFillableAmount.isGreaterThan(0) && isValidSignature) {
        // Order is fillable
    }

    // Create a metatransaction
    // We'll encode the the fillRfqOrder call to be signed in the metaTx wrapper
    const fillRfqOrderCalldata = contractWrappers.exchangeProxy
        .fillRfqOrder(rfqOrder, makerSignature, takerTokenAmount)
        .getABIEncodedTransactionData();
    const metaTxFields: MetaTransactionFields = {
        chainId: NETWORK_CONFIGS.chainId,
        verifyingContract: exchangeProxyAddress,
        signer: taker,
        sender,
        minGasPrice: new BigNumber(TX_DEFAULTS.gasPrice),
        maxGasPrice: new BigNumber(TX_DEFAULTS.gasPrice),
        // In theory this could be different, but we'll use the same expiration
        // as the order.
        expirationTimeSeconds: randomExpiration,
        salt: new BigNumber(Date.now()),
        callData: fillRfqOrderCalldata,
        value: new BigNumber(0),
        // A few can be specified in terms of any erc20 token
        // to go to the sender, but we'll leave it feeless
        feeToken: NULL_ADDRESS,
        feeAmount: new BigNumber(0),
    };
    const metaTx = new MetaTransaction(metaTxFields);
    const metaTxSignature = await metaTx.getSignatureWithProviderAsync(web3Wrapper.getProvider());

    const metaTxHash = await contractWrappers.exchangeProxy.getMetaTransactionHash(metaTx).callAsync();

    // Fill the Order via 0x Exchange Proxy contract
    txHash = await contractWrappers.exchangeProxy
        .executeMetaTransaction(metaTx, metaTxSignature)
        .sendTransactionAsync({
            from: sender,
            ...TX_DEFAULTS,
        });
    txReceipt = await printUtils.awaitTransactionMinedSpinnerAsync('executeMetaTransaction', txHash);
    printUtils.printTransaction('executeMetaTransaction', txReceipt, [['Meta-Transaction hash', metaTxHash], ['RFQ order hash', orderHash]]);

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
