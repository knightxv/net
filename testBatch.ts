import { Dchat } from "@bfchain/bnrtc2-dchat-node";
import { DCHAT_DPORT_PREFIX, MessageState } from "@bfchain/bnrtc2-dchat-typings";
import { Bnrtc2Buffer } from "@bfchain/bnrtc2-buffer";
import { Bnrtc2Controller } from "@bfchain/bnrtc2-client";

const DCHAT_DPORT = DCHAT_DPORT_PREFIX + "bfchainBatch";
let num = 20;
let startPort = 10000;
let peers: Peer[] = [];

class Peer {
    public dchat: Dchat;
    public controller: Bnrtc2Controller;
    public port: number;
    public address: string;
    public receiveCnt: number = 0;
    public sendCnt: number = 0;
    public sendFailedCnt: number = 0;


    constructor(address: string, port: number) {
        this.address = address;
        this.port = port;
        this.dchat = new Dchat("127.0.0.1", this.port);
        this.controller = new Bnrtc2Controller("127.0.0.1", this.port);
    }

    public async init() {
        await this.controller.bindAddress(this.address)
        await this.dchat.onMessage(DCHAT_DPORT, (address: string, dport: string, data: Uint8Array) => {
            this.receiveCnt++;
            return true;
        });
    }

    public async send(msg: string) {
        const pererAddress = peers[Math.floor(Math.random() * num)].address;
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
    peers.push(new Peer(address, startPort + i));
}

(async function run() {
    let j = 0;
    for (let i = 0; i < peers.length; i++) {
        let peer = peers[i];
        await peer.init();
    }

    for (let i = 0; i < peers.length; i++) {
        let peer = peers[i];
        setInterval((p: Peer,)=>{
            p.send("hello-"+(j++));
        }, 1000, peer);
    }
})();