/// <reference lib="dom"/>
import { Dchat } from "@bfchain/bnrtc2-dchat-node";
import { READY_STATE } from "@bfchain/bnrtc2-client-typings";

import {
    DCHAT_DPORT_PREFIX,
    MessageState,
} from "@bfchain/bnrtc2-dchat-typings";
import { Bnrtc2Buffer } from "@bfchain/bnrtc2-buffer";

const DCHAT_DPORT = DCHAT_DPORT_PREFIX + "bfchain";

const addresses = [
    "KPYCJZKbUBtvpv4pr6UHbzLo2X7LUtRHU",
    "MQ9Dj5xcuK2hamTjucivXmgSyuJHF31ew",
    "5n1BR4q2S2qNyRbXFcEvTiqLQN37xrp2U",
    "4QiEkLrekRLupzhPK71A1vZtvfvCwid5t",
    "BydYy3JYcUVnhMuMy1d88qcSRRLE6M5fk",
    "5N2xSkensNnphrM72dEExCengAwVZJpbX",
    "Mhg7XhH6Xafb4VnUKSqGoyok4HqMxNcgy",
    "5VM1wwQauoNwtzLCDTraCrpz7Lcc21S7q",
    "BiC4ELURiyfvXd8Vi3if4MWKhTEubLJnk",
    "8zpzvHYrKPc3Sz6aJhhjStCuiEVQbXqsb",
    "GfmSKKEJ1LYVUx6T2kRf13Q8kUSrWPBCh",
    "JKE8y3nbSHyUNgVaQaKbLHBLW1edm15QA",
    "AAtAdULG2HFa18dFBUxhXhW2YsNZWdv9N",
    "7hVkQkatGhmCSGUmNiTGiYHemMGziXpxg",
    "EnR11GRrtGpzfhHjGfaXpSiQbF9Rxkjuu",
    "2jbGgTnMaXGzVBZ4GGdxQvAyy7TyjbvET",
    "5QrLn4Gqf2QfAnUic85a4rTn3EmSQKs4s",
    "C4reZkHG945uSqHJvnQjNc9yBTFQcnXtH",
    "EhUUGrUCxc9pVuAe3F1eJbGvSk2f3iLzE",
    "HDXyLKFNSdgL9uM3KwQmdjWQsbFN6jZzo",
    "3jui8ko762DHguaNcfoHgJkWvCWsUo4ok",
    "3qDbNKccx3yfVdKWP9MKbGeAv5QRXRRdh",
    "NxsVmqYD1MkoTt25gyHeLvrTmjsYkYfgs",
    "5HtdLPq9wkEqf3DJRXov3FwAgzb5TWyMF",
];

let sendSuccessCount = 0;
let sendCount = 0;
let recvCount = 0;

function sendMsg(dchat: Dchat, address: string) {
    const pererAddress =
        addresses[Math.floor(Math.random() * addresses.length)];
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
for (let i = 0; i < addresses.length; i++) {
    const localAddress = addresses[i];
    const friends = addresses.slice(0, i).concat(addresses.slice(i + 1));
    const dchat = new Dchat("127.0.0.1", startPort + i);
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

    setTimeout(() => {
        sendMsg(dchat, localAddress);
    }, Math.ceil(30000 * Math.random()));

    setInterval(() => {
        sendMsg(dchat, localAddress);
    }, Math.ceil(30000 * Math.random()) + 30000);
}
