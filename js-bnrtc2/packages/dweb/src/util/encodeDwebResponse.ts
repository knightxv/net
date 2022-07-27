import type { ResponseMsg } from "../types";
import { encodeUTF8ToBinary } from "@bfchain/util-encoding-utf8";

export const encodeDwebResponse = (res: ResponseMsg) => {
  const headerStrBytes = encodeUTF8ToBinary(JSON.stringify(res.header));
  const dataBytesLength = 8 + headerStrBytes.byteLength + res.data.byteLength;
  const buf = new Uint8Array(dataBytesLength);
  const dv = new DataView(buf.buffer);
  let offset = 0;
  dv.setUint32(offset, res.reqId);
  offset += 4;
  dv.setUint16(offset, res.statusCode);
  offset += 2;
  dv.setUint16(offset, headerStrBytes.byteLength);
  offset += 2;
  buf.set(headerStrBytes, offset);
  offset += headerStrBytes.byteLength;
  buf.set(res.data, offset);
  return buf;
};
