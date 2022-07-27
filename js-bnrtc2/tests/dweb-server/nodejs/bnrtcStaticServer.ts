/// <reference types="node" />
import * as path from "path";
import { DwebServer } from "@bfchain/bnrtc2-dweb-node";
const dwebServer = new DwebServer();

const webRoot = path.resolve(
  __dirname,
  process.argv.find((arg) => arg.startsWith("-d="))?.slice(3) ?? "./www"
);
console.log("web root:", webRoot);

dwebServer.use(dwebServer.static(webRoot));
dwebServer.listen("www.baidu.com");
