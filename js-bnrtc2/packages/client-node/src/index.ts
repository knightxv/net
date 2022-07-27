import { bnrtc2Global } from "@bfchain/bnrtc2-client";
import ws from "ws";
import nodeFetch from "node-fetch";
bnrtc2Global.Channel = ws as any;
bnrtc2Global.fetch = nodeFetch as any;

export * from "@bfchain/bnrtc2-client";
