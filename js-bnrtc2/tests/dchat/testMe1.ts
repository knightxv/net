/// <reference lib="dom"/>
import { Dchat } from "@bfchain/bnrtc2-dchat-node";
import { READY_STATE } from "@bfchain/bnrtc2-client-typings";
import * as fs from "fs";

import {
    DCHAT_DPORT_PREFIX,
    MessageState,
} from "@bfchain/bnrtc2-dchat-typings";
import { Bnrtc2Buffer } from "@bfchain/bnrtc2-buffer";

const DCHAT_DPORT = DCHAT_DPORT_PREFIX + "bfchain";

let sendSuccessCount = 0;
let sendCount = 0;
let recvCount = 0;
let addressNum = 400;
let targetAddress = "0";
let addresses: string[] = [];
for (let i = 0; i < addressNum; i++) {
    addresses.push(i.toString())
}

function sendMsg(dchat: Dchat, address: string) {
    // const pererAddress =
    //    addresses[Math.floor(Math.random() * addresses.length)];
    const pererAddress = targetAddress;
    if (pererAddress === address) {
        return;
    }
    const msg = "I am " + sendCount;
    const buf = Bnrtc2Buffer.create(0);
    buf.putStr(msg);
    dchat.send(pererAddress, DCHAT_DPORT, buf.data()).then((resCode) => {
        if (resCode == MessageState.Success) {
            sendSuccessCount++;
            console.log(
                "%s send msg '%s' to  %s, total Sendcount %d(%d) RecvCount %d",
                address,
                msg,
                pererAddress,
                sendCount,
                sendSuccessCount,
                recvCount
            );
        } else {
            console.log(
                "%s send msg '%s' to  %s err %s ",
                address,
                msg,
                pererAddress,
                resCode
            );
        }
    });
    sendCount++;
}

let startPort = 20000;

let validPorts = readPorts();
function readPorts(): Set<number> {
    const data = fs.readFileSync('./portInfo.json', 'utf8');
    // parse JSON string to JSON object
    const ports = JSON.parse(data);
    const portSet = new Set<number>();
    for (const port of ports) {
        portSet.add(port);
    }

    return portSet;
}

function isValidPort(port: number): boolean {
    return validPorts.size === 0 || validPorts.has(port);
}

for (let i = 0; i < addresses.length; i++) {
    const port = startPort + i;
    if (!isValidPort(port)) {
        continue;
    }
    const localAddress = addresses[i];
    const friends = addresses.slice(0, i).concat(addresses.slice(i + 1));
    const dchat = new Dchat("127.0.0.1", port);
    dchat.onMessage(
        DCHAT_DPORT,
        (
            address: string,
            dport: string,
            data: Uint8Array,
            devid?: string,
            src?: string,
            isSync?: boolean
        ) => {
            const msg = Bnrtc2Buffer.from(data).pullStr();
            recvCount++;
            console.log(
                "Address %s got Peer %s message: %s, total Sendcount %d(%d) RecvCount %d",
                localAddress,
                address,
                msg,
                sendCount,
                sendSuccessCount,
                recvCount
            );
            return true;
        }
    );
    dchat.onStateChange(async function (state: READY_STATE) {
        if (state == READY_STATE.OPEN) {
            await dchat.login(localAddress);
            await dchat.addFriends(localAddress, friends);
        }
    });
    if (i != 0) {
        setTimeout(() => {
            sendMsg(dchat, localAddress);
        }, Math.ceil(50000 * Math.random()));

        setInterval(() => {
            sendMsg(dchat, localAddress);
        }, Math.ceil(50000 * Math.random()) + 50000);
    }
}
