import { Bnrtc2Controller } from "@bfchain/bnrtc2-client-node";

(async () => {
    const controller = new Bnrtc2Controller();
    const info = await controller.getAddressInfo("3jui8ko762DHguaNcfoHgJkWvCWsUo4ok");
    console.log(info);
})()
