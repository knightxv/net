import { Bnrtc2Client } from "@bfchain/bnrtc2-client-node";
import { stringToUint8Array } from "./util";

export function newPeer(opts: {
  host: string;
  port: number;
  to: string;
  recv?: boolean;
  send?: boolean;
}) {
  const client = new Bnrtc2Client(opts.host, opts.port);

  return {
    async run(cfg: { count: number }) {
      return new Promise<number>(async (resolve) => {
        let recvCount = 0;
        let startTime = 0;
        let endTime = 0;
        const { count } = cfg;
        if (opts.recv) {
          client.onData("HELLO", (x) => {
            if (recvCount === 0) {
              startTime = Date.now();
            }
            recvCount++;

            if (recvCount === count) {
              endTime = Date.now();
              const elapsedSeconds = (endTime - startTime) / 1000;
              const mps = Math.round(count / elapsedSeconds);
              resolve(mps);
            }
          });
        } else {
          resolve(0);
        }
        if (opts.send) {
          for (let i = 0; i < count; i++) {
            const x = stringToUint8Array(`${i}`);
            const u8a = new Uint8Array(1024).fill(0);
            u8a.set(x, 0);
            try {
              await client.send(opts.to, "HELLO", u8a);
            } catch (e) {
              console.log(`err: `, e);
            }
          }
        }
      });
    },
  };
}
