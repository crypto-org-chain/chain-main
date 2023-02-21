import { ContractWrappers, ERC20TokenContract } from '@0x/contract-wrappers';
import { OrderStatus, RfqOrder, SignatureType } from '@0x/protocol-utils';
import { BigNumber, hexUtils } from '@0x/utils';
import { Web3Wrapper } from '@0x/web3-wrapper';

import { NETWORK_CONFIGS, TX_DEFAULTS } from '../configs';
import { DECIMALS, UNLIMITED_ALLOWANCE_IN_BASE_UNITS } from '../constants';
import { PrintUtils } from '../print_utils';
import { providerEngine } from '../provider_engine';
import { getRandomFutureDateInSeconds, runMigrationsOnceIfRequiredAsync } from '../utils';

/**
 * In this scenario, the maker has separate addresses for holding funds
 * and signing orders. This example uses a normal Ethereum address for
 * holding funds, but this could also be a contract wallet.
 *
 * To allow a separate address to sign orders, the maker uses the
 * registerAllowedOrderSigner function on the 0x Exchange Proxy.
 *
 * The taker takes this order and fills it via the 0x Exchange Proxy.
 */
export async function scenarioAsync(): Promise<void> {
    await runMigrationsOnceIfRequiredAsync();
    PrintUtils.printScenario('Fill ERC20 RFQ Order With Order Signer');
    // Initialize the ContractWrappers, this provides helper functions around calling
    // 0x contracts as well as ERC20/ERC721 token contracts on the blockchain
    const contractWrappers = new ContractWrappers(providerEngine, { chainId: NETWORK_CONFIGS.chainId });
    // Initialize the Web3Wrapper, this provides helper functions around fetching
    // account information, balances, general contract logs
    const web3Wrapper = new Web3Wrapper(providerEngine);
    const [makerWallet, taker, makerOrderSigner] = await web3Wrapper.getAvailableAddressesAsync();
    const exchangeProxyAddress = contractWrappers.contractAddresses.exchangeProxy;
    const zrxTokenAddress = contractWrappers.contractAddresses.zrxToken;
    const etherTokenAddress = contractWrappers.contractAddresses.etherToken;
    const printUtils = new PrintUtils(
        web3Wrapper,
        contractWrappers,
        { makerWallet, taker, makerOrderSigner },
        { WETH: etherTokenAddress, ZRX: zrxTokenAddress },
    );
    printUtils.printAccounts();

    // The maker wallet needs to approve the signer address to sign orders
    // on its behalf.
    const registerOrderSignerTxHash = await contractWrappers.exchangeProxy
        // true for allow, false to revoke
        .registerAllowedOrderSigner(makerOrderSigner, true)
        .sendTransactionAsync({from: makerWallet});
    await printUtils.awaitTransactionMinedSpinnerAsync('MakerWallet registers order signer', registerOrderSignerTxHash);

    // the amount the maker is selling of maker asset
    const makerAssetAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(5), DECIMALS);
    // the amount the maker wants of taker asset
    const takerAssetAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(0.1), DECIMALS);

    // Allow the 0x Exchange Proxy to move ZRX on behalf of the maker wallet
    const erc20Token = new ERC20TokenContract(zrxTokenAddress, providerEngine);
    const makerZRXApprovalTxHash = await erc20Token
        .approve(exchangeProxyAddress, UNLIMITED_ALLOWANCE_IN_BASE_UNITS)
        .sendTransactionAsync({ from: makerWallet, ...TX_DEFAULTS });
    await printUtils.awaitTransactionMinedSpinnerAsync('MakerWallet ZRX Approval', makerZRXApprovalTxHash);

    // Allow the 0x Exchange Proxy to move WETH on behalf of the taker
    const etherToken = contractWrappers.weth9;
    const takerWETHApprovalTxHash = await etherToken
        .approve(exchangeProxyAddress, UNLIMITED_ALLOWANCE_IN_BASE_UNITS)
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
    const pool = hexUtils.leftPad(1);

    // Create the order
    const rfqOrder: RfqOrder = new RfqOrder({
        chainId: NETWORK_CONFIGS.chainId,
        verifyingContract: exchangeProxyAddress,
        maker: makerWallet,
        taker,
        makerToken: zrxTokenAddress,
        takerToken: etherTokenAddress,
        makerAmount: makerAssetAmount,
        takerAmount: takerAssetAmount,
        txOrigin: taker,
        expiry: randomExpiration,
        pool,
        salt: new BigNumber(Date.now()),
    });

    // Print order
    printUtils.printOrder(rfqOrder);

    // Print out the Balances and Allowances
    await printUtils.fetchAndPrintContractAllowancesAsync(exchangeProxyAddress);
    await printUtils.fetchAndPrintContractBalancesAsync();

    // Generate the order hash and sign it
    const signature = await rfqOrder.getSignatureWithProviderAsync(web3Wrapper.getProvider(), SignatureType.EthSign, makerOrderSigner);

    const [
        { orderHash, status },
        remainingFillableAmount,
        isValidSignature,
    ] = await contractWrappers.exchangeProxy.getRfqOrderRelevantState(rfqOrder, signature).callAsync();
    if (status === OrderStatus.Fillable && remainingFillableAmount.isGreaterThan(0) && isValidSignature) {
        // Order is fillable
    }

    // Fill the Order via 0x Exchange Proxy contract
    const txHash = await contractWrappers.exchangeProxy
        .fillRfqOrder(rfqOrder, signature, takerAssetAmount)
        .sendTransactionAsync({
            from: taker,
            ...TX_DEFAULTS,
        });
    const txReceipt = await printUtils.awaitTransactionMinedSpinnerAsync('fillRfqOrder', txHash);
    printUtils.printTransaction('fillRfqOrder', txReceipt, [['orderHash', orderHash]]);

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
