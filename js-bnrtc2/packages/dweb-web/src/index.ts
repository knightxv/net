import "@bfchain/bnrtc2-client-web";
export * from "@bfchain/bnrtc2-dweb";

import { DwebServer as BaseDwebServer } from "@bfchain/bnrtc2-dweb";

import {
  StaticFileReader,
  StaticOption,
  serverStatic,
} from "@bfchain/bnrtc2-dweb";

type IFileStat = {
  size: number;
};

export class DwebServer extends BaseDwebServer {
  static(relativeFileDir: string, options?: StaticOption) {
    const fileReader = new NodeJsStaticFileReader();
    return serverStatic(fileReader, relativeFileDir, options);
  }
}
class NodeJsStaticFileReader implements StaticFileReader {
  public fileRelativePathMap: { [key in string]: IFileStat | undefined } = {};
  async stat(root: string, relativeFile: string): Promise<IFileStat> {
    const filePath = `${root}${relativeFile}`;
    const fileStat = this.fileRelativePathMap[filePath];
    if (fileStat == undefined) {
      const buf = await fetch(filePath).then((res) => res.arrayBuffer());
      return (this.fileRelativePathMap[filePath] = { size: buf.byteLength });
    }
    return fileStat;
  }
  readFile(
    root: string,
    relativeFile: string,
    startPos?: number,
    endPos?: number
  ): Promise<Uint8Array> {
    return new Promise(async (resolve, reject) => {
      const filePath = `${root}${relativeFile}`;
      const buf = await fetch(filePath).then((res) => res.arrayBuffer());
      const fileData = new Uint8Array(buf);
      this.fileRelativePathMap[filePath] = { size: buf.byteLength };
      try {
        resolve(fileData.slice(startPos, endPos));
      } catch (err) {
        reject(err);
      }
    });
  }
}
