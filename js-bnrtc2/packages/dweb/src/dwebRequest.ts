import type { MethodType, DwebRequestMsg } from "./types";
import { decodeBinaryToUTF8 } from "@bfchain/util-encoding-utf8";

export class DwebRequest {
  reqId: number;
  path: string;
  method: MethodType;
  header: { [key in string]: string };
  postForm?: { [key in string]: string };
  query: { [key in string]: string };
  useData: { [key in string]: string } = {};
  constructor(bytes: Uint8Array) {
    const dataStr = decodeBinaryToUTF8(bytes);
    const requestMsg = JSON.parse(dataStr) as DwebRequestMsg;
    this.reqId = requestMsg.reqId;
    this.path = requestMsg.path;
    this.method = requestMsg.method;
    this.header = requestMsg.header;
    this.postForm = requestMsg.postForm;
    this.query = this.parseQuery(requestMsg.path);
  }
  parseQuery(query: string) {
    const searchMarkIndex = query.indexOf("?");
    if (searchMarkIndex < 0) {
      return {};
    }
    query = query.slice(searchMarkIndex + 1);
    var reg = /([^=&\s]+)[=\s]([^=&\s]+)/g;
    var obj: { [key in string]: string } = {};
    while (reg.exec(query)) {
      obj[RegExp.$1] = RegExp.$2;
    }
    return obj;
  }
}
