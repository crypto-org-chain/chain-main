import {
    ContractWrappers,
    DecodedLogEvent,
    IndexedFilterValues,
    IZeroExEvents,
    IZeroExRfqOrderFilledEventArgs,
} from '@0x/contract-wrappers';

import { NETWORK_CONFIGS } from '../configs';
import { PrintUtils } from '../print_utils';
import { providerEngine } from '../provider_engine';
import { runMigrationsOnceIfRequiredAsync } from '../utils';

/**
 * In this scenario, we will subscribe to exchange proxy RFQ order filled events
 */
export async function scenarioAsync(): Promise<void> {
    PrintUtils.printScenario('Exchange Proxy Subscribe');
    await runMigrationsOnceIfRequiredAsync();
    // Initialize the ContractWrappers, this provides helper functions around calling
    // 0x contracts as well as ERC20/ERC721 token contracts on the blockchain
    const contractWrappers = new ContractWrappers(providerEngine, { chainId: NETWORK_CONFIGS.chainId });
    // No filter, get all of the Fill Events
    const filterValues: IndexedFilterValues = {};
    // Subscribe to the Fill Events on the Exchange
    contractWrappers.exchangeProxy.subscribe(
        IZeroExEvents.RfqOrderFilled,
        filterValues,
        (err: null | Error, decodedLogEvent?: DecodedLogEvent<IZeroExRfqOrderFilledEventArgs>) => {
            if (err) {
                console.log('error:', err);
                providerEngine.stop();
            } else if (decodedLogEvent) {
                const fillLog = decodedLogEvent.log;
                PrintUtils.printData('Rfq Order Filled Event', [
                    ['orderHash', fillLog.args.orderHash],
                    ['maker', fillLog.args.maker],
                    ['taker', fillLog.args.taker],
                    ['makerTokenFilledAmount', fillLog.args.makerTokenFilledAmount.toString()],
                    ['takerTokenFilledAmount', fillLog.args.takerTokenFilledAmount.toString()],
                    ['makerToken', fillLog.args.makerToken],
                    ['takerToken', fillLog.args.takerToken],
                ]);
            }
        },
    );
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
