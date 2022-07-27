import "babel-polyfill";

import { DwebServer } from "@bfchain/bnrtc2-dweb-web";
import { Bnrtc2Client } from "@bfchain/bnrtc2-client";
import { DEFAULT_BASE_API_HOSTNAME } from "@bfchain/bnrtc2-client-api";
(async () => {
  const client = new Bnrtc2Client(DEFAULT_BASE_API_HOSTNAME, 19888);
  const dwebServer = new DwebServer(client);
  dwebServer.listen();
  const downloadManager = await dwebServer.getDownLoadManger();
  downloadManager.on("pageDownload", async (stream) => {
    console.log("pageDownload");
    const fileSize = await stream.getFileSize();
    const dprogress = document.getElementById("progress");
    const dstart = document.getElementById("start");
    dstart?.addEventListener("click", () => {
      stream.start();
    });
    const dpause = document.getElementById("pause");
    dpause?.addEventListener("click", () => {
      stream.pause();
    });
    const dReStart = document.getElementById("reStart");
    dReStart?.addEventListener("click", () => {
      stream.reStart();
    });
    const dStop = document.getElementById("stop");
    dStop?.addEventListener("click", () => {
      stream.stop();
    });
    stream.on("data", (data) => {
      if (!dprogress) {
        return;
      }
      if (fileSize) {
        dprogress.innerText = (stream.progress / fileSize).toFixed(2) + "%";
      } else {
        dprogress.innerText = "" + stream.progress;
      }
    });
    stream.on("end", () => {
      console.log("pageDownload end process");
    });
    stream.on("error", () => {
      console.log("pageDownload err");
    });
  });
  // const stream = await downloadManager.downloadStream(
  //   "http://326574386a476231346641667a51424e674c5248637756784c643450696631.6d6e3a7777772e62616964752e636f6d.localhost:19333/bfchain.apk"
  // );
  // console.log("pageDownload", {
  //   size: stream.size,
  // });
  // stream.on("data", (data) => {
  //   console.log("data get data size", data.byteLength);
  //   console.log("data proress", stream.progress);
  // });
  // stream.on("end", () => {
  //   console.log("pageDownload end process");
  // });
  // stream.on("error", () => {
  //   console.log("pageDownload err");
  // });
})();
