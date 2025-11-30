import { ContractWrappers, ERC20TokenContract } from '@0x/contract-wrappers';
import {
    encodeFillQuoteTransformerData,
    encodeWethTransformerData,
    FillQuoteTransformerOrderType,
    FillQuoteTransformerSide,
    findTransformerNonce,
    RfqOrder,
    SignatureType,
} from '@0x/protocol-utils';
import { BigNumber, hexUtils } from '@0x/utils';
import { Web3Wrapper } from '@0x/web3-wrapper';

import { NETWORK_CONFIGS, TX_DEFAULTS } from '../configs';
import {
    DECIMALS,
    ETH_ADDRESS,
    NULL_ADDRESS,
    UNLIMITED_ALLOWANCE_IN_BASE_UNITS,
} from '../constants';
import { PrintUtils } from '../print_utils';
import { providerEngine } from '../provider_engine';
import { getRandomFutureDateInSeconds, runMigrationsOnceIfRequiredAsync } from '../utils';

/**
 * In this scenario, we use the TransformERC20 function to convert ETH
 * to ZRX.
 *
 * TransformERC20 is used for aggregating various liquidity sources
 * into a single trade. We only use a single RFQ order in this trade,
 * but TransformERC20 can be used to fill quotes from a variety of
 * DEX protocols (e.g. Uniswap, Balancer) in a single trade.
 */
export async function scenarioAsync(): Promise<void> {
    await runMigrationsOnceIfRequiredAsync();
    PrintUtils.printScenario('Transform ERC20');
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

    // the amount of ether the taker is selling
    const etherAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(0.1), DECIMALS);

    // Print out the Balances and Allowances
    await printUtils.fetchAndPrintContractAllowancesAsync(contractWrappers.contractAddresses.exchangeProxy);
    await printUtils.fetchAndPrintContractBalancesAsync();

    // Create an RFQ order to fill later
    const makerAssetAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(5), DECIMALS);
    const zrxToken = new ERC20TokenContract(zrxTokenAddress, providerEngine);
    const makerZRXApprovalTxHash = await zrxToken
        .approve(exchangeProxyAddress, UNLIMITED_ALLOWANCE_IN_BASE_UNITS)
        .sendTransactionAsync({ from: maker, ...TX_DEFAULTS });
    await printUtils.awaitTransactionMinedSpinnerAsync('Maker ZRX Approval', makerZRXApprovalTxHash);
    const randomExpiration = getRandomFutureDateInSeconds();
    const pool = hexUtils.leftPad(1);
    const rfqOrder: RfqOrder = new RfqOrder({
        chainId: NETWORK_CONFIGS.chainId,
        verifyingContract: exchangeProxyAddress,
        maker,
        taker: NULL_ADDRESS,
        makerToken: zrxTokenAddress,
        takerToken: etherTokenAddress,
        makerAmount: makerAssetAmount,
        takerAmount: etherAmount,
        txOrigin: taker,
        expiry: randomExpiration,
        pool,
        salt: new BigNumber(Date.now()),
    });
    const signature = await rfqOrder.getSignatureWithProviderAsync(web3Wrapper.getProvider(), SignatureType.EthSign, maker);

    // TRANSFORMATIONS

    // In this case the taker is starting with ETH.
    // In order to fill an RFQ order, we need to transform the ETH
    // into WETH first.
    // We do this via the WethTransformer.
    const wethTransformerData = {
        deploymentNonce: findTransformerNonce(
            contractWrappers.contractAddresses.transformers.wethTransformer,
            contractWrappers.contractAddresses.exchangeProxyTransformerDeployer,
        ),
        data: encodeWethTransformerData({
            token: ETH_ADDRESS,
            amount: etherAmount,
        }),
    };

    // The second step is using the WETH to fill an RFQ order
    // via the FillQuoteTransformer.
    const fillQuoteTransformerData = {
        deploymentNonce: findTransformerNonce(
            contractWrappers.contractAddresses.transformers.fillQuoteTransformer,
            contractWrappers.contractAddresses.exchangeProxyTransformerDeployer,
        ),
        data: encodeFillQuoteTransformerData({
            side: FillQuoteTransformerSide.Sell,
            sellToken: etherTokenAddress,
            buyToken: zrxTokenAddress,
            rfqOrders: [{
                order: rfqOrder,
                signature,
                maxTakerTokenFillAmount: etherAmount,
            }],
            bridgeOrders: [],
            limitOrders: [],
            fillSequence: [FillQuoteTransformerOrderType.Rfq],
            refundReceiver: NULL_ADDRESS,
            fillAmount: etherAmount,
        }),
    };

    // Call TransformERC20 on the 0x Exchange Proxy contract
    const txHash = await contractWrappers.exchangeProxy
        .transformERC20(
            ETH_ADDRESS,
            zrxTokenAddress,
            etherAmount,
            makerAssetAmount,
            [wethTransformerData, fillQuoteTransformerData],
        )
        .sendTransactionAsync({
            from: taker,
            value: etherAmount,
            ...TX_DEFAULTS,
        });
    const txReceipt = await printUtils.awaitTransactionMinedSpinnerAsync('TransformERC20', txHash);
    printUtils.printTransaction('TransformERC20', txReceipt, []);

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
