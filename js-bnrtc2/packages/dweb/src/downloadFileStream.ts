import type { DownloadFileInfo } from "./types";
import { EventEmitter } from "@bfchain/util-event";
import { bnrtc2Global } from "@bfchain/bnrtc2-client-api";
import { mediaTypeMap } from "./plugin/mediaTypeMap";

enum DownloadStatus {
  UN_START,
  PAUSE,
  DOWNLOADING,
  STOP,
  END,
}
type DownloadFileStreamEvent = {
  data: [Uint8Array];
  end: [];
  stop: [];
  pause: [];
  reStart: [];
  error: [Error];
};
export class DownloadFileStream extends EventEmitter<DownloadFileStreamEvent> {
  constructor(public fileInfo: DownloadFileInfo) {
    super();
  }
  public status: DownloadStatus = DownloadStatus.UN_START;
  public progress: number = this.fileInfo.start || 0;
  get url() {
    return this.fileInfo.url;
  }
  header?: Headers;
  requestHeader() {
    if (this.header) {
      return this.header;
    }
    return bnrtc2Global
      .fetch(this.url, {
        method: "HEAD",
      })
      .then((res) => {
        this.header = res.headers;
        return res.headers;
      });
  }
  getMimeType() {
    const fileTypeReg = /\.(\w+)(\?.+)?$/;
    const fileMatch = this.url.match(fileTypeReg);
    if (fileMatch) {
      const fileType = fileMatch[1];
      const supportContentType = mediaTypeMap[`.${fileType}`];
      return supportContentType;
    }
  }
  async supportResume() {
    const header = await this.requestHeader();
    return header.get("Accept-Ranges") === "bytes";
  }
  async getFileSize() {
    const header = await this.requestHeader();
    const cl = header.get("Content-Length");
    return cl ? +cl : null;
  }
  start() {
    if (this.status !== DownloadStatus.UN_START) {
      throw new ReferenceError("download is start");
    }
    this._downloadFile();
  }
  async _downloadFile() {
    if (this.status === DownloadStatus.DOWNLOADING) {
      return;
    }
    this.status = DownloadStatus.DOWNLOADING;
    const supportResume = await this.supportResume();
    if (!supportResume) {
      this._fetchDownload();
      return;
    }
    const fileSize = await this.getFileSize();
    if (fileSize == null) {
      this.emit(
        "error",
        new ReferenceError("support resume but fileSize is null")
      );
      return;
    }
    while (this.status === DownloadStatus.DOWNLOADING) {
      const dataBuf = await bnrtc2Global
        .fetch(this.fileInfo.url, {
          headers: {
            Range: `bytes=${this.progress}-`,
          },
        })
        .then((res) => {
          if (res.status !== 206) {
            throw new ReferenceError("response status is not 206");
          }
          return res.arrayBuffer();
        })
        .catch((err) => {
          this.emit("error", err);
          throw err;
        });
      this.progress += dataBuf.byteLength;
      this.emit("data", new Uint8Array(dataBuf));
      if (this.progress >= fileSize) {
        this.status = DownloadStatus.END;
        this.emit("end");
        return;
      }
    }
  }
  _fetchDownload() {
    const self = this;
    this.progress = 0;
    let abortController: AbortController | undefined = undefined;
    if ("AbortController" in window) {
      abortController = new AbortController();
    }
    bnrtc2Global
      .fetch(this.url, { signal: abortController?.signal })
      .then((res) => {
        const reader = res.body?.getReader();
        if (reader == undefined) {
          return;
        }
        new ReadableStream({
          start(controller) {
            function push() {
              reader!.read().then(({ done, value }) => {
                if (done) {
                  self.status = DownloadStatus.END;
                  self.emit("end");
                  // 告诉浏览器已经结束数据发送
                  controller.close();
                  return;
                }
                self.progress += value!.byteLength;
                self.emit("data", value!);
                // 取得数据并将它通过controller发送给浏览器
                controller.enqueue(value);
                if (self.status === DownloadStatus.DOWNLOADING) {
                  push();
                } else {
                  abortController?.abort();
                  reader?.cancel();
                }
              });
            }
            push();
          },
        });
      })
      .catch((err) => {
        this.emit("error", err);
      });
  }
  stop() {
    if (this.status === DownloadStatus.STOP) {
      return;
    }
    this.status = DownloadStatus.STOP;
    this.emit("stop");
  }
  async pause() {
    if (this.status !== DownloadStatus.DOWNLOADING) {
      return;
    }
    this.status = DownloadStatus.PAUSE;
    this.emit("pause");
  }
  reStart() {
    if (this.status === DownloadStatus.PAUSE) {
      this._downloadFile();
      this.emit("reStart");
      return;
    }
    if (this.status === DownloadStatus.STOP) {
      this.progress = 0;
      this._downloadFile();
      this.emit("reStart");
      return;
    }
    if (this.status === DownloadStatus.END) {
      this.progress = 0;
      this._downloadFile();
      this.emit("reStart");
      return;
    }
  }
}
