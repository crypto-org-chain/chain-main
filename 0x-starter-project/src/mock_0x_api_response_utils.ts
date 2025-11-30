import { ContractWrappers, ERC20TokenContract } from '@0x/contract-wrappers';
import { RfqOrder, SignatureType } from '@0x/protocol-utils';
import { Web3ProviderEngine } from '@0x/subproviders';
import { BigNumber, hexUtils } from '@0x/utils';
import { Web3Wrapper } from '@0x/web3-wrapper';
import { AxiosInstance } from 'axios';
import AxiosMockAdapter from 'axios-mock-adapter';
import * as HttpStatus from 'http-status-codes';

import { NETWORK_CONFIGS, TX_DEFAULTS } from './configs';
import { DECIMALS, MOCK_0x_API_BASE_URL, UNLIMITED_ALLOWANCE_IN_BASE_UNITS } from './constants';
import { getRandomFutureDateInSeconds } from './utils';

/**
 * Set up mock responses to hypothetical 0x API calls.
 *
 */
export async function setUpMock0xApiResponsesAsync(
    axiosInstance: AxiosInstance,
    maker: string,
    taker: string,
    providerEngine: Web3ProviderEngine,
): Promise<void> {
    const axiosMock = new AxiosMockAdapter(axiosInstance);

    await setUpWethZrxMockResponseAsync(axiosMock, providerEngine, maker, taker);
}

/**
 * Set up a mock swap/quote response to buy ZRX with WETH
 *
 */
async function setUpWethZrxMockResponseAsync(
    axiosMock: AxiosMockAdapter,
    providerEngine: Web3ProviderEngine,
    maker: string,
    taker: string,
): Promise<void> {
    const contractWrappers = new ContractWrappers(providerEngine, { chainId: NETWORK_CONFIGS.chainId });
    const web3Wrapper = new Web3Wrapper(providerEngine);
    const exchangeProxyAddress = contractWrappers.contractAddresses.exchangeProxy;

    const zrxTokenAddress = contractWrappers.contractAddresses.zrxToken;
    const zrkToken = new ERC20TokenContract(zrxTokenAddress, providerEngine);
    const etherTokenAddress = contractWrappers.contractAddresses.etherToken;
    const makerAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(5), DECIMALS);
    const takerAmount = Web3Wrapper.toBaseUnitAmount(new BigNumber(0.1), DECIMALS);

    // make sure maker has an allowance
    await zrkToken
        .approve(exchangeProxyAddress, UNLIMITED_ALLOWANCE_IN_BASE_UNITS)
        .awaitTransactionSuccessAsync({ from: maker, ...TX_DEFAULTS });

    // Create the order
    const randomExpiration = getRandomFutureDateInSeconds();
    const pool = hexUtils.leftPad(1);

    const rfqOrder: RfqOrder = new RfqOrder({
        chainId: NETWORK_CONFIGS.chainId,
        verifyingContract: exchangeProxyAddress,
        maker,
        taker,
        makerToken: zrxTokenAddress,
        takerToken: etherTokenAddress,
        makerAmount,
        takerAmount,
        txOrigin: taker,
        expiry: randomExpiration,
        pool,
        salt: new BigNumber(Date.now()),
    });

    // Generate the order hash and sign it
    const signature = await rfqOrder.getSignatureWithProviderAsync(
        web3Wrapper.getProvider(),
        SignatureType.EthSign,
        maker,
    );

    const callData = contractWrappers.exchangeProxy
        .fillRfqOrder(rfqOrder, signature, takerAmount)
        .getABIEncodedTransactionData();

    const mockWethZrxResponse = {
        chainId: NETWORK_CONFIGS.chainId,
        // If buyAmount was specified in the request it provides the price of
        // buyToken in sellToken and vice versa. This price does not include
        // the slippage provided in the request above, and therefore represents
        // the best possible price.
        price: makerAmount.div(takerAmount).toString(),
        // The price which must be met or else the entire transaction will
        // revert. This price is influenced by the slippagePercentage parameter.
        // On-chain sources may encounter price movements from quote to settlement.
        // This quote will be totally RFQ, returning the order price.
        guaranteedPrice: makerAmount.div(takerAmount).toString(),
        to: exchangeProxyAddress,
        data: callData,
        value: '0',
        gas: '200000',
        estimatedGas: '200000',
        from: taker,
        gasPrice: TX_DEFAULTS.gasPrice.toString(),
        protocolFee: '0',
        minimumProtocolFee: '0',
        buyTokenAddress: zrxTokenAddress,
        sellTokenAddress: etherTokenAddress,
        buyAmount: makerAmount.toString(),
        sellAmount: takerAmount.toString(),
        sources: [
            {
                name: '0x',
                proportion: '1',
            },
        ],
        allowanceTarget: exchangeProxyAddress,
    };

    axiosMock.onGet(`${MOCK_0x_API_BASE_URL}/swap/v1/quote`).reply(HttpStatus.StatusCodes.OK, mockWethZrxResponse);
}
