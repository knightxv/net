/// <reference types="node" />
import "@bfchain/bnrtc2-client-node";
export * from "@bfchain/bnrtc2-dweb";
import { DwebServer as BaseDwebServer } from "@bfchain/bnrtc2-dweb";

import {
  StaticFileReader,
  StaticOption,
  serverStatic,
} from "@bfchain/bnrtc2-dweb";
import * as path from "path";
import * as fs from "fs/promises";
import * as ofs from "fs";
export class DwebServer extends BaseDwebServer {
  static(relativeFileDir: string, options?: StaticOption) {
    const fileReader = new NodeJsStaticFileReader();
    return serverStatic(fileReader, relativeFileDir, options);
  }
}
class NodeJsStaticFileReader implements StaticFileReader {
  stat(root: string, relativeFile: string): Promise<{ size: number }> {
    const fileName = path.join(root, relativeFile);
    return fs.stat(fileName);
  }
  readFile(
    root: string,
    relativeFile: string,
    startPos?: number,
    endPos?: number
  ): Promise<Uint8Array> {
    return new Promise(async (resolve, reject) => {
      const fileName = path.join(root, relativeFile);
      try {
        if (startPos == undefined) {
          startPos = 0;
          if (endPos == undefined) {
            const data = await fs.readFile(fileName);
            resolve(new Uint8Array(data));
            return;
          }
        }
        if (endPos == undefined) {
          const { size } = await this.stat(root, relativeFile);
          endPos = size;
        }
        if (startPos >= endPos) {
          resolve(new Uint8Array());
          return;
        }
        const dataBuf = new Uint8Array(endPos - startPos);
        let offset = 0;
        var fReadStream = ofs.createReadStream(fileName, {
          encoding: "binary",
          start: startPos,
          end: endPos,
          autoClose: true,
        });
        fReadStream.on("data", function (chunk) {
          const cdata = chunk as Buffer;
          dataBuf.set(cdata, offset);
          offset += cdata.byteLength;
        });
        fReadStream.on("end", function () {
          resolve(dataBuf);
        });
        fReadStream.on("error", reject);
      } catch (err) {
        reject(err);
      }
    });
  }
}
