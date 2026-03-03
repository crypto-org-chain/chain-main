import { ContractWrappers } from '@0x/contract-wrappers';
import { BigNumber } from '@0x/utils';
import { Web3Wrapper } from '@0x/web3-wrapper';
import axios from 'axios';

import { NETWORK_CONFIGS, TX_DEFAULTS } from '../configs';
import { DECIMALS, MOCK_0x_API_BASE_URL, UNLIMITED_ALLOWANCE_IN_BASE_UNITS } from '../constants';
import { setUpMock0xApiResponsesAsync } from '../mock_0x_api_response_utils';
import { PrintUtils } from '../print_utils';
import { providerEngine } from '../provider_engine';
import { runMigrationsOnceIfRequiredAsync } from '../utils';

/**
 * In this scenario, the taker grabs a swap quote from 0x API
 * and fills it on-chain.
 *
 * The taker wants to buy 5 ZRX
 */
export async function scenarioAsync(): Promise<void> {
    await runMigrationsOnceIfRequiredAsync();
    PrintUtils.printScenario('Fill a 0x API Swap Quote');

    // Taker wishes to buy 5 ZRX with WETH
    const buyAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(5), DECIMALS);

    // SETUP

    // create an axios instance for http requests
    const axiosInstance = axios.create();
    // Initialize the ContractWrappers for setting up our taker's token balance
    const contractWrappers = new ContractWrappers(providerEngine, { chainId: NETWORK_CONFIGS.chainId });
    // Initialize the Web3Wrapper, this provides helper functions around fetching
    // account information, balances, general contract logs
    const web3Wrapper = new Web3Wrapper(providerEngine);
    // Grabbing a maker address as well for setting up our mock API response
    const [maker, taker] = await web3Wrapper.getAvailableAddressesAsync();
    // Call util for mocking an API responses
    await setUpMock0xApiResponsesAsync(axiosInstance, maker, taker, providerEngine);

    const exchangeProxyAddress = contractWrappers.contractAddresses.exchangeProxy;
    const zrxTokenAddress = contractWrappers.contractAddresses.zrxToken;
    const wethTokenAddress = contractWrappers.contractAddresses.etherToken;
    const printUtils = new PrintUtils(
        web3Wrapper,
        contractWrappers,
        { taker },
        { WETH: wethTokenAddress, ZRX: zrxTokenAddress },
    );
    printUtils.printAccounts();

    // Allow the 0x Exchange Proxy to move WETH on behalf of the taker
    const etherToken = contractWrappers.weth9;
    const takerWETHApprovalTxHash = await etherToken
        .approve(exchangeProxyAddress, UNLIMITED_ALLOWANCE_IN_BASE_UNITS)
        .sendTransactionAsync({ from: taker, ...TX_DEFAULTS });
    await printUtils.awaitTransactionMinedSpinnerAsync('Taker WETH Approval', takerWETHApprovalTxHash);

    // Convert ETH into WETH for taker by depositing ETH into the WETH contract
    const takerAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(0.1), DECIMALS);
    const takerWETHDepositTxHash = await etherToken.deposit().sendTransactionAsync({
        from: taker,
        value: takerAmount,
    });
    await printUtils.awaitTransactionMinedSpinnerAsync('Taker WETH Deposit', takerWETHDepositTxHash);

    PrintUtils.printData('Setup', [
        ['Taker WETH Approval', takerWETHApprovalTxHash],
        ['Taker WETH Deposit', takerWETHDepositTxHash],
    ]);

    // CREATE REQUEST FOR 0x API

    // An API key in the header gives access to RFQ liqudity
    // Ask the 0x labs team for RFQ access for your application
    const quoteRequestHeaders = {
        '0x-api-key': 'a-dope-api-key',
    };
    const quoteRequestParams = {
        // REQUIRED PARAMETERS //

        // token address the taker wants to buy
        buyToken: zrxTokenAddress,

        // token address the taker wants to sell
        sellToken: wethTokenAddress,

        // here we specify the amount of the buy token, since
        // the taker is requesting to buy 5 ZRX
        buyAmount: buyAmount.toString(),

        // alternatively the taker could have asked to sell a certain
        // amount of WETH for ZRX
        // sellAmount: Web3Wrapper.toBaseUnitAmount(new BigNumber(0.1), DECIMALS)

        // OPTIONAL PARAMETERS //

        // Not all parameters are listed here
        // see https://0x.org/docs/api#get-swapv1quote

        // The maximum acceptable slippage from the quoted price to execution price.
        // E.g 0.03 for 3% slippage allowed.
        slippagePercentage: 0.01,

        // The address which will fill the quote.
        // When provided the gas will be estimated and returned and the
        // entire transaction will be validated for success.
        // If the validation fails a Revert Error will be returned
        // in the response.
        // Required for RFQ liquidity
        takerAddress: taker,

        // Required for RFQ liquidity,
        // see https://0x.org/docs/guides/rfqt-in-the-0x-api#maker-endpoint-interactions
        intentOnFilling: true,
    };

    // Make the request to 0x API
    const swapResponse = await axiosInstance.get(`${MOCK_0x_API_BASE_URL}/swap/v1/quote`, {
        params: quoteRequestParams,
        headers: quoteRequestHeaders,
    });

    // Grab necessary data for the transaction
    // from the response from 0x API
    const { data, to, value, gas, gasPrice } = swapResponse.data;

    // Send the swap quote in a transaction
    // This example is using 0x's web3wrapper, but this could be done
    // with ethers or web3.js as well.
    const swapTxHash = await web3Wrapper.sendTransactionAsync({ from: taker, to, data, value, gas, gasPrice });
    const txReceipt = await printUtils.awaitTransactionMinedSpinnerAsync('fill 0x API swap', swapTxHash);

    printUtils.printTransaction('fill 0x API swap', txReceipt);

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
