import { DCHAT_DPORT_PREFIX, DCHAT_SERVICE_CLIENT_DPORT, DCHAT_SERVICE_SERVER_DPORT, DCHAT_DPORT_LEN_MAX, DCHAT_ADDRESS_LEN_MAX, DCHAT_MESSAGE_TIMEOUT_MAX, DCHAT_MESSAGE_TIMEOUT_MIN } from "@bfchain/bnrtc2-dchat-typings";
import { Bnrtc2Buffer } from "@bfchain/bnrtc2-buffer";

export function isValidAddress(address: string): boolean {
    if (address.length > DCHAT_ADDRESS_LEN_MAX) {
        return false;
    }

    return true;
}

export function isValidDport(dport: string, isSend: boolean): boolean {
    if (isSend && dport == DCHAT_SERVICE_SERVER_DPORT) {
        return true;
    }

    if (!isSend && dport == DCHAT_SERVICE_CLIENT_DPORT) {
        return true;
    }

    if (dport.length > DCHAT_DPORT_LEN_MAX) {
        return false;
    }

    return dport.startsWith(DCHAT_DPORT_PREFIX);
}

export function isValidMessageTimeout(timeout: number) {
    return ((timeout > DCHAT_MESSAGE_TIMEOUT_MAX) || (timeout < DCHAT_MESSAGE_TIMEOUT_MIN));
}

export function stringSetToBuffer(set: Set<string>): Bnrtc2Buffer {
    const buf = Bnrtc2Buffer.create(0);
    set.forEach((value) => {
        buf.pushStr(value);
    });
    return buf;
}

export function getByteArray(buf: Bnrtc2Buffer): Array<Uint8Array> {
    const arr: Array<Uint8Array> = [];
    while (buf.length > 0) {
        const u8a = buf.pull();
        if (u8a.length == 0) {
            break;
        }
        arr.push(u8a);
    }
    return arr;
}
export function getStringArray(buf: Bnrtc2Buffer): Array<string> {
    const arr: Array<string> = [];
    while (buf.length > 0) {
        const str = buf.pullStr();
        if (str === "") {
            break;
        }
        arr.push(str);
    }
    return arr;
}