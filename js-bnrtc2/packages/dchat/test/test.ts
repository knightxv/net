/// <reference lib="dom"/>
import { Dchat } from "../src";
import {
  DCHAT_DPORT_PREFIX,
  MessageState,
} from "@bfchain/bnrtc2-dchat-typings";
import {
  encodeUTF8ToBinary,
  decodeBinaryToUTF8,
} from "@bfchain/util-encoding-utf8";

const DCHAT_DPORT = DCHAT_DPORT_PREFIX + "bfchain";
const DCHAT_ADDRESS = "JustAddress";
const message1 = "我是测试[I am test]";
const message2 = "我是测试2[I am test2]";
function logIt(...args: any[]) {
  console.log(args);
}

const dchat1 = new Dchat();
const dchat2 = new Dchat();
function messageHandler1(
  address: string,
  dport: string,
  data: Uint8Array
): boolean {
  const msg = decodeBinaryToUTF8(data);
  logIt("dchat1 got message: ", address, dport, msg);
  if (address == DCHAT_ADDRESS && dport == DCHAT_DPORT && msg == message2) {
    logIt("dchat1 recv message2 ok");
  }
  return true;
}
function messageHandler2(
  address: string,
  dport: string,
  data: Uint8Array
): boolean {
  const msg = decodeBinaryToUTF8(data);
  logIt("dchat2 got message: ", address, dport, msg);
  if (address == DCHAT_ADDRESS && dport == DCHAT_DPORT && msg == message1) {
    logIt("dchat2 recv message1 ok");
  }
  return true;
}
dchat1.onMessage("dawad", messageHandler1).then((ret) => {
  if (ret != false) {
    throw new Error("dport ERROR");
  }
});

let p = dchat1.send(
  "DCHAT_ADDRESS55555555555555555555555555555555555555555555555555555",
  DCHAT_DPORT,
  new Uint8Array()
);
p.then((state: string) => {
  if (state !== MessageState.FormatError) {
    throw new Error("address ERROR");
  }
});
p = dchat1.send("DCHAT_ADDRESS55555", "DCHAwT_DPORT", new Uint8Array());
p.then((state: string) => {
  if (state !== MessageState.FormatError) {
    throw new Error("dport ERROR");
  }
});

dchat1.onMessage(DCHAT_DPORT, messageHandler1);
dchat2.onMessage(DCHAT_DPORT, messageHandler2);

const msg1 = encodeUTF8ToBinary(message1);
const msg2 = encodeUTF8ToBinary(message2);

const p1 = dchat1.send(DCHAT_ADDRESS, DCHAT_DPORT, msg1);
const p2 = dchat2.send(DCHAT_ADDRESS, DCHAT_DPORT, msg2);
p1.then((state: string) => {
  if (state === MessageState.Success) {
    logIt("dchat1 send message1 ok");
  } else {
    logIt("dchat1 send message1 failed", state);
  }
});
p2.then((state: string) => {
  if (state === MessageState.Success) {
    logIt("dchat2 send message2 ok");
  } else {
    logIt("dchat2 send message2 failed", state);
  }
});
