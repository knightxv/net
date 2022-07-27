import { DwebServer } from "@bfchain/bnrtc2-dweb-node";
import { Bnrtc2Client } from "@bfchain/bnrtc2-client";

(async () => {
  const server = new DwebServer(new Bnrtc2Client("127.0.0.1", 19888));
  const url = await server.dwebResolve({
    host: "www.baidu.com",
    address: "2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn",
  });
  console.log(url);
})();
