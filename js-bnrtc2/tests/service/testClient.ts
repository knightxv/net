import { Bnrtc2Client } from "@bfchain/bnrtc2-client-node";
import { BnrtcClient, MessageState } from "@bfchain/bnrtc2-service";

const bnrtc2Client = new Bnrtc2Client("127.0.0.1", 19888);

class SignServer {
    sign() {
        console.log("server run sign server");
        return "success";
    }
}

(async () => {
    const client = new BnrtcClient(bnrtc2Client);
    const bindResult = await client.bindAddress("12xgwBgtrCGNQN1buFrBVpCpBbs");
    if (bindResult !== MessageState.Success) {
        throw new ReferenceError("bind address fail");
    }
    const remoteSignServer = await client.getRemoteApiService<SignServer>(
        "abc"
    );
    if (!remoteSignServer) {
        throw new ReferenceError("getRemoteApiService fail");
    }
    try {
        const signResult = await remoteSignServer.sign();
        console.log("client sign result:", signResult);
    } catch (err) {
        console.log("client sign error", err);
    }
})();
