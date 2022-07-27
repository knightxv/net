import * as http from "http";
import { Buffer } from "buffer";
import * as url from "url";
import "@bfchain/bnrtc2-client-typings";
import {
  IBnrtcClient,
  DwebServer,
  IDwebHandler,
  IBnrtcController,
} from "@bfchain/bnrtc2-dweb-node";
import { StatefulAttacher } from "@bfcs/util-evt-stateful";
const HTTP_PORT = 3000;

let httpRes: any;

class TestController implements IBnrtcController {
  async getServiceInfo(): Promise<Bnrtc2.Bnrtc2ServiceInfo | undefined> {
    return {
      port: 19222,
    };
  }
}

class HttpClient implements IBnrtcClient {
  eventMap: Map<string, Bnrtc2.Bnrtc2DataHandler> = new Map();
  controller: TestController = new TestController();
  send(address: string, dport: string, buffer: Uint8Array): void {
    if (httpRes) {
      httpRes.write(Buffer.from(buffer));
      httpRes.end();
      httpRes = undefined;
    }
  }
  onData(dport: string, handler: Bnrtc2.Bnrtc2DataHandler): void {
    this.eventMap.set(dport, handler);
  }
  offData(dport: string, handler: Bnrtc2.Bnrtc2DataHandler): void {
    this.eventMap.delete(dport);
  }
  trigger(dport: string, data: Bnrtc2.Bnrtc2Data) {
    const handler = this.eventMap.get(dport);
    if (!handler) {
      return;
    }
    handler(data);
  }

  onOpen: StatefulAttacher<boolean> = new StatefulAttacher<boolean>(true);
}
const httpClient = new HttpClient();
const dwebServer = new DwebServer(httpClient);
// dwebServer.get("/", (req, res) => {
//   res.setAssertType("html");
//   res.end('<div style="color: red">node js server</div>');
// });
dwebServer.use(dwebServer.static("./nodejs/dist"));
dwebServer.listen("www.baidu.com");

const app = http.createServer((req: any, res: any) => {
  httpRes = res;
  const dataList: Buffer[] = [];
  req.on("data", function (chunk: Buffer) {
    dataList.push(chunk);
  });
  req.on("end", function () {
    const params = url.parse(req.url, true).query;
    const address = params.address as string;
    const dport = params.dPort as string;
    const data: Bnrtc2.Bnrtc2Data = {
      address,
      dport,
      devid: "",
      dst: "",
      data: Uint8Array.from(Buffer.concat(dataList)),
      isSync: false,
    };
    httpClient.trigger(dport, data);
  });
  // res.writeHead(200, { "Content-Type": "text/plain" });
  // res.end("okay");
});

app.listen(HTTP_PORT, "localhost", () => {
  console.log("start server::" + HTTP_PORT);
});
