import { Dchat } from "@bfchain/bnrtc2-dchat-node";
import { DCHAT_DPORT_PREFIX, MessageState } from "@bfchain/bnrtc2-dchat-typings";
import { READY_STATE } from "@bfchain/bnrtc2-client-typings";
import { Bnrtc2Buffer } from "@bfchain/bnrtc2-buffer";
import { Bnrtc2Controller } from "@bfchain/bnrtc2-client";

const DCHAT_DPORT = DCHAT_DPORT_PREFIX + "bfchainBatch";
let num = 20;
let startPort = 10000;
let peers: Peer[] = [];

class Peer {
    public dchat: Dchat;
    public port: number;
    public address: string;
    public friends: string[] = [];
    public receiveCnt: number = 0;
    public sendCnt: number = 0;
    public sendFailedCnt: number = 0;


    constructor(address: string, friends: string[], port: number) {
        this.address = address;
        this.port = port;
        this.friends = friends;
        this.dchat = new Dchat("127.0.0.1", this.port);
    }

    public async init() {
        const self = this
        this.dchat.onMessage(DCHAT_DPORT, (address: string, dport: string, data: Uint8Array) => {
            self.receiveCnt++;
            return true;
        });
        this.dchat.onStateChange(async function (state: READY_STATE) {
            if (state == READY_STATE.OPEN) {
                await self.dchat.login(self.address);
                await self.dchat.addFriends(self.address, self.friends);
            }
        });
    }

    public async send(msg: string) {
        const pererAddress = peers[1 + Math.floor(Math.random() * (num - 1))].address;
        if (pererAddress === this.address) {
            return;
        }
        const buf = Bnrtc2Buffer.create(0);
        buf.putStr(msg);
        await this.dchat.send(pererAddress, DCHAT_DPORT, buf.data()).then((resCode)=>{
            if (resCode == MessageState.Success) {
                this.sendCnt++;
                console.log("%s send msg '%s' to  %s ok ", this.address, msg, pererAddress);

            } else {
                console.log("%s send msg '%s' to  %s err %s ", this.address, msg, pererAddress, resCode);
                this.sendFailedCnt++;
            }
        });
    }
}

for (let i = 0; i < num; i++) {
    const address = "batchTest" + String([i]);
    const friends = [];
    for (let j = 0; j < num; j++) {
        if (i != j) {
            friends.push("batchTest" + String([j]));
        }
    }
    peers.push(new Peer(address, friends, startPort + i));
}

(async function run() {
    let j = 0;
    for (let i = 1; i < peers.length; i++) {
        let peer = peers[i];
        await peer.init();
    }

    for (let i = 1; i < peers.length; i++) {
        let peer = peers[i];
        setInterval((p: Peer,)=>{
            p.send("hello-"+(j++));
        }, 1000, peer);
    }
})();