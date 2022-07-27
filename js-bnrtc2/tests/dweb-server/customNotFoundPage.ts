import { DwebServer } from "@bfchain/bnrtc2-dweb-node";

(async () => {
  const server = new DwebServer();
  const res = await server.customNotFoundPage(new Uint8Array([52, 48, 52]));
  console.log(res.ok);
})();
