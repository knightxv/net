import "regenerator-runtime/runtime";
import {
  IDwebHandler,
  StaticFileReader,
  StaticOption,
  serverStatic,
} from "@bfchain/bnrtc2-dweb";
import { DwebServer } from "@bfchain/bnrtc2-dweb-web";
const fileRelativePathMap: { [key in string]: File } = {};
Reflect.set(self, "fileRelativePathMap", fileRelativePathMap);
class WebStaticFileReader implements StaticFileReader {
  async stat(root: string, relativeFile: string): Promise<{ size: number }> {
    console.log("stat");
    const file = fileRelativePathMap[`${root}${relativeFile}`];
    const fileData = await file.arrayBuffer();
    return { size: fileData.byteLength };
  }
  async readFile(
    root: string,
    relativeFile: string,
    start: number,
    end: number
  ): Promise<Uint8Array> {
    console.log("readFile1");
    const file = fileRelativePathMap[`${root}${relativeFile}`];
    const fileData = await file.arrayBuffer();
    return new Uint8Array(fileData.slice(start, end));
  }
}
const dwebServer = new DwebServer();

function webStatic(
  relativeFileDir: string,
  options?: StaticOption
): IDwebHandler {
  const fileReader = new WebStaticFileReader();
  return serverStatic(fileReader, relativeFileDir, options);
}
dwebServer.use(webStatic(""));
dwebServer.listen("www.baidu.com");
(() => {
  const uploadBtn = document.getElementById("upload");
  if (!uploadBtn) return;
  uploadBtn.onclick = () => {
    const filesList = document.getElementById("file");
    if (filesList == null) {
      return;
    }
    const domFile = filesList as HTMLInputElement;
    const files = domFile.files;
    if (files == null) {
      return;
    }
    for (let i = 0; i <= files.length - 1; i++) {
      const file = files.item(i);
      if (file == null) {
        continue;
      }
      const fileKey = file.webkitRelativePath.replace(/^[\w-]+/, "");
      fileRelativePathMap[fileKey] = file;
    }
  };
})();
