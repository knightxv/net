import { bnrtc2Global } from "@bfchain/bnrtc2-client";

bnrtc2Global.Channel = WebSocket.bind(self) as any;
bnrtc2Global.fetch = fetch.bind(self) as any;

export * from "@bfchain/bnrtc2-client";
