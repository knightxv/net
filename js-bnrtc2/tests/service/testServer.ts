import { Bnrtc2Client } from "@bfchain/bnrtc2-client-node";
import { BnrtcClient, MessageState } from "@bfchain/bnrtc2-service";

const bnrtc2Client = new Bnrtc2Client("127.0.0.1", 19020);

class SignServer {
    async sign() {
        console.log("server run sign server");
        return "success";
    }
}

(async () => {
    const serverClient = new BnrtcClient(bnrtc2Client);
    const bindResult = await serverClient.bindAddress(
        "7VtbQvoPF26p7NHEMNehXJtcFCs"
    );
    if (bindResult !== MessageState.Success) {
        throw new ReferenceError("bind address fail");
    }
    const registResult = await serverClient.registerRemoteApiService(
        "abc",
        new SignServer()
    );
    console.log("serverClient registResult:", registResult);
})();
