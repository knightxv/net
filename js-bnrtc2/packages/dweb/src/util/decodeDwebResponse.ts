import type { ResponseMsg } from "../types";
import { decodeBinaryToUTF8 } from "@bfchain/util-encoding-utf8";

export const decodeDwebResponse = (buffer: Uint8Array): ResponseMsg => {
  const dataView = new DataView(buffer.buffer);
  let offset = 0;
  const reqId = dataView.getUint32(offset);
  offset += 4;
  const statusCode = dataView.getUint16(offset);
  offset += 2;
  const headerLength = dataView.getUint16(offset);
  offset += 2;
  const headerJsonStr = buffer.slice(offset, offset + headerLength);
  offset += headerLength;
  const header = JSON.parse(decodeBinaryToUTF8(headerJsonStr));
  const data = buffer.slice(offset);
  return {
    reqId,
    statusCode,
    header,
    data,
  };
};
