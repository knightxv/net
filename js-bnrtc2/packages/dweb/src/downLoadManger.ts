import { EventEmitter } from "@bfchain/util-event";
import { bnrtc2Global } from "@bfchain/bnrtc2-client-api";
import { DownloadFileStream } from "./downloadFileStream";
import type { pageDownloadEvData, DownloadFileInfo } from "./types";

type DownLoadMangerEvent = {
  pageDownload: [DownloadFileStream];
  close: [];
};

export class DownLoadManger extends EventEmitter<DownLoadMangerEvent> {
  constructor(public dPort: string, public dwebPort: number) {
    super();
    this.initWs();
  }
  ws?: Bnrtc2.Channel;
  initWs() {
    const wsUrl = `ws://localhost:${this.dwebPort}/connectWs?dPort=${this.dPort}`;
    const ws = new bnrtc2Global.Channel(wsUrl);
    this.ws = ws;
    ws.onopen = () => {
      ws.onmessage = (ev) => {
        let message: pageDownloadEvData = ev.data;
        if (typeof ev.data === "string") {
          message = JSON.parse(ev.data) as pageDownloadEvData;
        }
        if (message == undefined) {
          return;
        }
        if (message.type === "pageDownload") {
          this.emit("pageDownload", new DownloadFileStream(message));
        }
      };
    };
  }
  close() {
    this.ws?.close();
    this.emit("close");
  }
  async downloadStream(url: string, start = 0) {
    const fileInfo: DownloadFileInfo = {
      url,
      start,
    };
    const fileStream = new DownloadFileStream(fileInfo);
    fileStream.start();
    return fileStream;
  }
}
