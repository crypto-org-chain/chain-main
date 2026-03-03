import { providerEngine } from '../provider_engine';

import { scenarioAsync as cancelPairLimitOrders } from './cancel_pair_limit_orders';
import { scenarioAsync as executeMetatransactionFillRfqOrder } from './execute_metatransaction_fill_rfq_order';
import { scenarioAsync as fill0xApiSwap } from './fill_0x_api_swap';
import { scenarioAsync as fillERC20LimitOrder } from './fill_erc20_limit_order';
import { scenarioAsync as fillERC20OtcOrder } from './fill_erc20_otc_order';
import { scenarioAsync as fillERC20RfqOrder } from './fill_erc20_rfq_order';
import { scenarioAsync as fillERC20RfqOrderWithMakerOrderSigner } from './fill_erc20_rfq_order_with_maker_order_signer';
import { scenarioAsync as fillERC20TakerSignedOtcOrder } from './fill_erc20_taker_signed_otc_order';
import { scenarioAsync as transformERC20 } from './transform_erc20';

void (async () => {
    try {
        await fill0xApiSwap();
        await fillERC20LimitOrder();
        await cancelPairLimitOrders();
        await fillERC20RfqOrder();
        await fillERC20RfqOrderWithMakerOrderSigner();
        await fillERC20OtcOrder();
        await executeMetatransactionFillRfqOrder();
        await fillERC20TakerSignedOtcOrder();
        await transformERC20();
    } catch (e) {
        console.log(e);
        providerEngine.stop();
        process.exit(1);
    }
})();
