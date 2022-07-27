import test from "ava";
import "@bfchain/bnrtc2-client-typings";
import {
  decodeDwebResponse,
  DwebRequest,
  DwebRequestMsg,
  DwebServer,
  IBnrtcClient,
  IBnrtcController,
  MethodType,
  ResponseMsg,
} from "../src";
import { encodeUTF8ToBinary } from "@bfchain/util-encoding-utf8";
import { PromiseOut } from "@bfchain/util";
import {StatefulEvt} from '@bfcs/util-evt';

class TestController implements IBnrtcController {
  async getServiceInfo(): Promise<Bnrtc2.Bnrtc2ServiceInfo | undefined> {
    return {
      port: 19223,
    };
  }
}


class TestClient implements IBnrtcClient {
  onOpen = new StatefulEvt(false).attacher;
  controller: IBnrtcController = new TestController();
  handler?: Bnrtc2.Bnrtc2DataHandler;
  sendMessagePro?: PromiseOut<ResponseMsg>;
  send(address: string, dport: string, data: Uint8Array): void {
    this.sendMessagePro?.resolve(decodeDwebResponse(data));
  }
  onData(dport: string, handler: Bnrtc2.Bnrtc2DataHandler): void {
    this.handler = handler;
  }
  offData(dport: string, handler: Bnrtc2.Bnrtc2DataHandler): void {
    throw new Error("Method not implemented.");
  }
  trigger(request: DwebRequestMsg) {
    this.sendMessagePro = new PromiseOut();
    this.handler?.({
      address: "",
      dst: "",
      devid: "",
      dport: "dweb",
      data: encodeUTF8ToBinary(JSON.stringify(request)),
      isSync: false,
    });

    return this.sendMessagePro.promise;
  }

}

test("base dweb server", async (t) => {
  const testClient = new TestClient();
  const dwebServer = new DwebServer(testClient);
  dwebServer.listen("dweb");
  {
    const sendMessage = await testClient.trigger({
      reqId: 1,
      path: "/",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 1,
      statusCode: 404,
      header: {},
      data: new Uint8Array(),
    });
  }
  {
    dwebServer.get("/", (req, res) => {
      res.end("root");
    });
    const sendMessage = await testClient.trigger({
      reqId: 1,
      path: "/",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 1,
      statusCode: 200,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("root"),
    });
  }
  {
    dwebServer.get("/a", (req, res) => {
      res.status(500);
    });
    const sendMessage = await testClient.trigger({
      reqId: 1,
      path: "/a",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 1,
      statusCode: 500,
      header: {},
      data: encodeUTF8ToBinary(""),
    });
  }
});

test("can not repeact end response", async (t) => {
  const testClient = new TestClient();
  const dwebServer = new DwebServer(testClient);
  dwebServer.listen("dweb");
  {
    dwebServer.get("/", (req, res) => {
      res.end("root");
      try {
        res.end("root");
        t.fail();
      } catch (err) {
        t.pass();
      }
    });
    const sendMessage = await testClient.trigger({
      reqId: 1,
      path: "/",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 1,
      statusCode: 200,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("root"),
    });
  }
});

test("base post request", async (t) => {
  const testClient = new TestClient();
  const dwebServer = new DwebServer(testClient);
  dwebServer.listen("dweb");
  {
    const sendMessage = await testClient.trigger({
      reqId: 2,
      path: "/a",
      method: "POST",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 2,
      statusCode: 404,
      header: {},
      data: new Uint8Array(),
    });
    dwebServer.post("/a", (req, res) => {
      res.end("postMessage");
    });
    dwebServer.post("/a", (req, res) => {
      res.end("postMessage");
    });
  }
  {
    const sendMessage = await testClient.trigger({
      reqId: 2,
      path: "/a",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 2,
      statusCode: 404,
      header: {},
      data: encodeUTF8ToBinary(""),
    });
  }
  {
    const sendMessage = await testClient.trigger({
      reqId: 2,
      path: "/a",
      method: "POST",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 2,
      statusCode: 200,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("postMessage"),
    });
  }
});
test("base all request", async (t) => {
  const testClient = new TestClient();
  const dwebServer = new DwebServer(testClient);
  dwebServer.listen("dweb");
  dwebServer.all("/a", (req, res) => {
    res.end("ok");
  });
  const methodList: MethodType[] = ["GET", "POST", "DELETE", "PATCH", "PUT"];
  for (const method of methodList) {
    const sendMessage = await testClient.trigger({
      reqId: 2,
      path: "/a",
      method,
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 2,
      statusCode: 200,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("ok"),
    });
  }
});

test("server regexp handler", async (t) => {
  const testClient = new TestClient();
  const dwebServer = new DwebServer(testClient);
  dwebServer.listen("dweb");
  /// start with `/abc/`
  dwebServer.get(/^\/abc\//, (req, res) => {
    res.end("regexp");
  });
  const notFondReqList: DwebRequestMsg[] = [
    {
      reqId: 3,
      path: "/",
      method: "GET",
      header: {},
    },
    {
      reqId: 3,
      path: "/ab",
      method: "GET",
      header: {},
    },
    {
      reqId: 3,
      path: "/abc",
      method: "GET",
      header: {},
    },
    {
      reqId: 3,
      path: "/abcd",
      method: "GET",
      header: {},
    },
    {
      reqId: 3,
      path: "/abc/a",
      method: "POST",
      header: {},
    },
  ];
  for (const reqMsg of notFondReqList) {
    const sendMessage = await testClient.trigger(reqMsg);
    t.deepEqual(sendMessage, {
      reqId: 3,
      statusCode: 404,
      header: {},
      data: new Uint8Array(),
    });
  }
  const successReqList: DwebRequestMsg[] = [
    {
      reqId: 3,
      path: "/abc/a",
      method: "GET",
      header: {},
    },
    {
      reqId: 3,
      path: "/abc/bb",
      method: "GET",
      header: {},
    },
  ];
  for (const reqMsg of successReqList) {
    const sendMessage = await testClient.trigger(reqMsg);
    t.deepEqual(sendMessage, {
      reqId: 3,
      statusCode: 200,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("regexp"),
    });
  }
});
test("server mix path and regexp handler and call next function", async (t) => {
  const testClient = new TestClient();
  const dwebServer = new DwebServer(testClient);
  dwebServer.listen("dweb");
  dwebServer.get(/^\/abc\//, (req, res, next) => {
    res.status(504);
    next();
  });
  dwebServer.get("/abc/", (req, res) => {
    res.end("ok");
  });
  {
    const sendMessage = await testClient.trigger({
      reqId: 5,
      path: "/abc/",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 5,
      statusCode: 504,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("ok"),
    });
  }
  dwebServer.get(/^\/bfc\//, (req, res, next) => {
    res.status(504);
  });
  dwebServer.get("/bfc/", (req, res) => {
    res.end("ok");
  });
  {
    const sendMessage = await testClient.trigger({
      reqId: 5,
      path: "/bfc/",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 5,
      statusCode: 504,
      header: {},
      data: new Uint8Array(),
    });
  }
});

test("use plugin", async (t) => {
  const testClient = new TestClient();
  const dwebServer = new DwebServer(testClient);
  dwebServer.listen("dweb");
  // powser manage
  dwebServer.use((req, res, next) => {
    if (req.query["user"] === "admin") {
      next();
    } else {
      res.status(403);
      res.end();
    }
  });
  dwebServer.get("/", (req, res) => {
    res.end("ok");
  });
  {
    const sendMessage = await testClient.trigger({
      reqId: 6,
      path: "/",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 6,
      statusCode: 403,
      header: {},
      data: new Uint8Array(),
    });
  }
  {
    const sendMessage = await testClient.trigger({
      reqId: 6,
      path: "/?user=knight",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 6,
      statusCode: 403,
      header: {},
      data: new Uint8Array(),
    });
  }
  {
    const sendMessage = await testClient.trigger({
      reqId: 6,
      path: "/?user=admin",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 6,
      statusCode: 200,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("ok"),
    });
  }
});

test("use path match plugin next function", async (t) => {
  const testClient = new TestClient();
  const dwebServer = new DwebServer(testClient);
  dwebServer.listen("dweb");
  dwebServer.get("/abc", (req, res) => {
    res.end("ok");
  });
  {
    const sendMessage = await testClient.trigger({
      reqId: 6,
      path: "/abc?user=knight",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 6,
      statusCode: 200,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("ok"),
    });
  }
  dwebServer.use("/abc", (req, res, next) => {
    res.status(304);
    next();
  });
  {
    const sendMessage = await testClient.trigger({
      reqId: 6,
      path: "/abc?user=knight",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 6,
      statusCode: 304,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("ok"),
    });
  }
});

test("use path match plugin", async (t) => {
  const testClient = new TestClient();
  const dwebServer = new DwebServer(testClient);
  dwebServer.listen("dweb");
  dwebServer.use("/root", (req, res, next) => {
    res.end("admin");
  });
  dwebServer.get("/", (req, res) => {
    res.end("ok");
  });
  {
    const sendMessage = await testClient.trigger({
      reqId: 6,
      path: "/?user=knight",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 6,
      statusCode: 200,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("ok"),
    });
  }
  {
    const sendMessage = await testClient.trigger({
      reqId: 6,
      path: "/root",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 6,
      statusCode: 200,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("admin"),
    });
  }
  {
    const sendMessage = await testClient.trigger({
      reqId: 6,
      path: "/root",
      method: "POST",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 6,
      statusCode: 200,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("admin"),
    });
  }
  {
    const sendMessage = await testClient.trigger({
      reqId: 6,
      path: "/root/bc",
      method: "PATCH",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 6,
      statusCode: 200,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("admin"),
    });
  }
});

test("use after use plugin handler", async (t) => {
  const testClient = new TestClient();
  const dwebServer = new DwebServer(testClient);
  dwebServer.listen("dweb");
  dwebServer.get("/root", (req, res, next) => {
    res.status(504);
    next();
  });
  {
    const sendMessage = await testClient.trigger({
      reqId: 6,
      path: "/root",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 6,
      statusCode: 504,
      header: {},
      data: encodeUTF8ToBinary(""),
    });
  }
  dwebServer.afterUse("/root", (req, res, next) => {
    res.end("end");
  });
  {
    const sendMessage = await testClient.trigger({
      reqId: 6,
      path: "/root",
      method: "GET",
      header: {},
    });
    t.deepEqual(sendMessage, {
      reqId: 6,
      statusCode: 504,
      header: {
        "content-type": "text/plain",
      },
      data: encodeUTF8ToBinary("end"),
    });
  }
});

test("dwebResolve", async (t) => {
  const dwebServer = new DwebServer();
  {
    const url = await dwebServer.dwebResolve({
      host: "www.baidu.com",
      address: "2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn",
    });
    t.is(url, "http://dweb.61.localhost:19222/abc");
  }
});
