import { assertContentTypeMap } from "./const";
import type { AssertType, ResponseMsg } from "./types";
import { encodeDwebResponse } from "./util/encodeDwebResponse";
import { encodeUTF8ToBinary } from "@bfchain/util-encoding-utf8";
import type { IBnrtcClient } from "./types";
import type { ResponseHeader } from ".";

const DEFAULT_STATUS_CODE = 404;

export class DwebResponse {
  constructor(
    public bnrtcClient: IBnrtcClient,
    public reqId: number,
    public address: string,
    public dPort: string,
    public devid: string
  ) {}
  header: ResponseHeader = {};
  statusCode: number = DEFAULT_STATUS_CODE;
  isSend: boolean = false;
  setAssertType(assertType: AssertType) {
    this.setHeader("Content-Type", assertContentTypeMap[assertType]);
  }
  setHeader(name: string, value: string) {
    this.header[name] = value;
  }
  writeHead(statusCode: number, header: ResponseHeader) {
    this.statusCode = statusCode;
    this.header = {
      ...this.header,
      ...header,
    };
  }
  status(statusCode: number) {
    this.statusCode = statusCode;
    return {
      send: this.end,
    };
  }
  json(jsonData: Object) {
    const jsonStr = JSON.stringify(jsonData);
    this.setAssertType("json");
    this.end(jsonStr);
  }
  end = (data?: Uint8Array | string) => {
    if (this.isSend) {
      throw new ReferenceError("repeact send");
    }
    if (data != undefined) {
      if (this.statusCode === DEFAULT_STATUS_CODE) {
        this.status(200);
      }
      if (!this.header["Content-Type"]) {
        if (typeof data === "string") {
          this.setAssertType("text");
        } else {
          this.setAssertType("octetStream");
        }
      }
      if (typeof data === "string") {
        data = encodeUTF8ToBinary(data);
      }
    }
    const responseMsg = {
      reqId: this.reqId,
      statusCode: this.statusCode,
      header: this.header,
      data: data || new Uint8Array(),
    };
    const bytes = encodeDwebResponse(responseMsg);
    this.isSend = true;
    this.bnrtcClient.send(this.address, "DWEB", bytes, this.devid);
  };
}
