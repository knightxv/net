import { BnrtcClient, IBnrtc2Client } from "@bfchain/bnrtc2-service";
import type { RESULT_CODE } from "@bfchain/bnrtc2-client-typings";
import { EventEmitter } from "@bfchain/util-event";

const eventEmitter = new EventEmitter();

class LocalEvent implements IBnrtc2Client {
    async send(
        dst: string,
        dport: string,
        data: Uint8Array,
        devid: string = "",
        src: string = ""
    ): Promise<RESULT_CODE> {
        eventEmitter.emit(dport, { data } as never);

        return 0;
    }
    async onData(
        dport: string,
        handler: Bnrtc2.Bnrtc2DataHandler
    ): Promise<RESULT_CODE> {
        eventEmitter.addListener(dport, handler);
        return 0;
    }
    async offData(
        dport: string,
        handler?: Bnrtc2.Bnrtc2DataHandler
    ): Promise<RESULT_CODE> {
        eventEmitter.off(dport, handler);
        return 0;
    }
    controller = {
        async getServiceAddresses(serviceName: string) {
            return [];
        },
        async registService(serverInfo: any): Promise<boolean> {
            return true;
        },
        async bindAddress(address: string) {
            return true;
        },
    };
}

const server = new BnrtcClient(new LocalEvent());

class SignServer {
    sign() {
        console.log("server run sign server");
        return "success";
    }
}

server.registerRemoteApiService("abc", new SignServer());

(async () => {
    const client = new BnrtcClient(new LocalEvent());
    const remoteSignServer = await client.getRemoteApiService<SignServer>(
        "abc"
    );
    const signResult = await remoteSignServer?.sign();
    console.log("client sign result:", signResult);
})();
