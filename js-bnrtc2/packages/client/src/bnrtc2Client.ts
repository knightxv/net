import { Bnrtc2Api, DEFAULT_BASE_API_HOSTNAME, DEFAULT_BASE_API_PORT } from "@bfchain/bnrtc2-client-api";
import { PromiseOut, sleep, unsleep } from "@bfchain/util-extends-promise";
import { Evt, Attacher, StatefulEvt, StatefulAttacher } from "@bfcs/util-evt";
import {
  MESSAGE_TYPE,
  READY_STATE,
  RESULT_CODE,
} from "@bfchain/bnrtc2-client-typings";
import { Bnrtc2Buffer } from "@bfchain/bnrtc2-buffer";
import { EasyMap } from "@bfchain/util-extends-map";
import { Bnrtc2Controller } from "./bnrtc2Controller";
/**
 * Bnrtc客户端
 */

const BackPressureRequestTimeout: number = 30000; //  30s

let eventDataReader = (data: any) => data;
if (typeof Blob !== "undefined") {
  eventDataReader = (data) => {
    const reader = new FileReader();
    if (data instanceof Blob) {
      reader.readAsArrayBuffer(data);
      return new Promise<ArrayBuffer>((resolve, reject) => {
        reader.onload = (ev) => {
          if (ev.loaded === ev.total) {
            resolve(new Uint8Array(reader.result as ArrayBuffer));
          }
        };
        reader.onerror = reject;
      });
    }
    return data;
  }
}

export class Bnrtc2Client implements Bnrtc2.Bnrtc2Client {
  private readonly _api: Bnrtc2Api;
  readonly controller: Bnrtc2.Bnrtc2ControllerInterface;
  private _msgIdAcc: Uint32Array = new Uint32Array(1);
  constructor(apiHost: string = DEFAULT_BASE_API_HOSTNAME, apiPort: number = DEFAULT_BASE_API_PORT) {
    this._api = new Bnrtc2Api(apiHost, apiPort);
    this.controller = new Bnrtc2Controller(apiHost, apiPort);

    const readyStateEvt = new StatefulEvt(READY_STATE.CONNECTING);
    this.onReadyStateChanged = readyStateEvt.attacher;
    const onReadyStateChangedPoster = readyStateEvt.poster;

    const openEvt = new StatefulEvt(false);
    this.onOpen = openEvt.attacher;
    const onOpenPoster = openEvt.poster;

    const closeEvt = new StatefulEvt(false);
    this.onClose = closeEvt.attacher;
    const onClosePoster = closeEvt.poster;

    const errorEvt = new Evt<Error>();
    this.onError = errorEvt.attacher;
    const onErrorPoster = errorEvt.poster;

    const messageEvt = new Evt<Uint8Array>();
    this.onMessage = messageEvt.attacher;
    const onMessagePoster = messageEvt.poster;

    this.onReadyStateChanged.attach((readyState: READY_STATE) => {
      if (readyState === READY_STATE.OPEN) {
        if (this.onOpen.state === false) {
          onOpenPoster.post(true);
        }
      } else if (readyState === READY_STATE.CLOSE) {
        if (this.onClose.state === false) {
          onClosePoster.post(true);
        }
      }
    });

    const channel = this._api.connectChannel();
    channel.addEventListener("open", () => {
      onReadyStateChangedPoster.post(READY_STATE.OPEN);
    });

    channel.addEventListener("close", () => {
      onReadyStateChangedPoster.post(READY_STATE.CLOSE);
    });

    channel.addEventListener("error", (e: any) => {
      onErrorPoster.post(new Error(`rtc error ${e}`));
    });

    this.close = (code?: number, reason?: string) => {
      if (
        readyStateEvt.state === READY_STATE.CLOSING ||
        readyStateEvt.state === READY_STATE.CLOSE
      ) {
        return;
      }
      onReadyStateChangedPoster.post(READY_STATE.CLOSING);
      this._close(code, reason);
    };

    this.onOpen.attach(() => {
      this.isOpened = true;
      if (this._closeBeforeOpen != undefined) {
        this._close(this._closeBeforeOpen[0], this._closeBeforeOpen[1]);
      }
    });

    this.onClose.attach(() => {
      this.isOpened = false
      this.close();
    });

    channel.addEventListener("message", (msg: MessageEvent) => {
      onMessagePoster.post(msg.data);
    });
    this._channel = channel;

    this.onMessage.attach(this._runHandler.bind(this));
  }

  private isOpened: boolean = false;
  private onReadyStateChanged: StatefulAttacher<READY_STATE>;
  readonly onOpen: StatefulAttacher<boolean>;
  readonly onClose: StatefulAttacher<boolean>;
  readonly onError: Attacher<Error>;
  private onMessage: Attacher<Uint8Array>;
  private _channel: Bnrtc2.Channel;
  private _closeBeforeOpen?: [number | undefined, string | undefined];

  buildDataMsg(type: MESSAGE_TYPE, msgId: number, dport: string, src: string, dst: string, devid: string, buffer: Bnrtc2Buffer | Uint8Array | undefined, neighbors: number = 0): Uint8Array {
    if (buffer instanceof Uint8Array) {
      buffer = Bnrtc2Buffer.from(buffer);
    } else if (buffer == undefined) {
      buffer = Bnrtc2Buffer.create(0);
    }
    /*
     * -------------------------------------------------------------------------------------------------
     * |  u8   |  uint32 |  uint32   |  str  |   str   |   str   |    str    |   str    | bool | bytes |
     * -------------------------------------------------------------------------------------------------
     * | type  |  msgId  | neighbors | dport | srcAddr | dstAddr | srcDevid  | dstDevid | sync |  data |
     * -------------------------------------------------------------------------------------------------
     * */
    const srcDevId = "" // no srcDevId, but need to be filled
    buffer.putBool(false);
    buffer.putStr(devid);
    buffer.putStr(srcDevId);
    buffer.putStr(dst);
    buffer.putStr(src);
    buffer.putStr(dport);
    if (type == MESSAGE_TYPE.NEIGHBORCAST) {
      buffer.putU32(neighbors);
    }
    buffer.putU32(msgId);
    buffer.putU8(type);
    return buffer.data();
  }

  private _doSend(data: Uint8Array) {
    this._channel.send(data)
  }

  private async _send(type: MESSAGE_TYPE, dport: string, src: string, dst: string, devid: string, buffer: Bnrtc2Buffer | Uint8Array | undefined, neighbors: number = 0): Promise<RESULT_CODE> {
    if (!this.isOpened) {
      return RESULT_CODE.CONNECT_FAILED
    }

    const msgId = this._getMessageId();
    const task = this._msg2TaskMap.forceGet(msgId);
    const data = this.buildDataMsg(type, msgId, dport, src, dst, devid, buffer, neighbors);
    this._doSend(data);
    return task.promise;
  }

  async send(dst: string, dport: string, data: Bnrtc2Buffer | Uint8Array, devid: string = "", src: string = ""): Promise<RESULT_CODE> {
    return this._send(MESSAGE_TYPE.DATA, dport, src, dst, devid, data)
  }

  async sendAll(dst: string, dport: string, data: Bnrtc2Buffer | Uint8Array, devid: string = "", src: string = ""): Promise<RESULT_CODE> {
    return this._send(MESSAGE_TYPE.MULTICAST, dport, src, dst, devid, data)
  }

  async sendNeighbor(num: number, dport: string, data: Bnrtc2Buffer | Uint8Array, src: string = ""): Promise<RESULT_CODE> {
    return this._send(MESSAGE_TYPE.NEIGHBORCAST, dport, src, "", "", data, num)
  }

  async broadcast(dport: string, data: Bnrtc2Buffer | Uint8Array, src: string = ""): Promise<RESULT_CODE> {
    return this._send(MESSAGE_TYPE.BROADCAST, dport, src, "", "", data)

  }
  async sync(dport: string, data: Bnrtc2Buffer | Uint8Array, src: string = ""): Promise<RESULT_CODE> {
    return this._send(MESSAGE_TYPE.SYNC, dport, src, src, "", data)
  }

  close(code?: number, reason?: string) {
    this._closeBeforeOpen = [code, reason];
  }

  private _dport2FunctionsMap = new Map<
    string,
    Set<Bnrtc2.Bnrtc2DataHandler>
  >();
  private _msg2TaskMap = EasyMap.from<number, PromiseOut<RESULT_CODE>, number>({
    creater: (msgId: number) => {
      const task = new PromiseOut<RESULT_CODE>();
      const tickTimeout = sleep(BackPressureRequestTimeout, () => {
        task.reject(RESULT_CODE.TIMEOUT);
      });
      task.onFinished(() => { // 完成时删除自己
        unsleep(tickTimeout);
        this._msg2TaskMap.delete(msgId);
      });
      return task;
    }
  });

  private _getMessageId() {
    return this._msgIdAcc[0]++;
  }

  /**
   * 接收数据监听事件
   * @param dport
   * @param handler
   * @returns
   */
  async onData(dport: string, handler: Bnrtc2.Bnrtc2DataHandler): Promise<RESULT_CODE> {
    // 保存dport与handler的映射关系
    let funcSet = this._dport2FunctionsMap.get(dport);
    if (!funcSet) {
      funcSet = new Set();
      funcSet.add(handler);
      this._dport2FunctionsMap.set(dport, funcSet);
      return this._sendBindDportMsg(dport);
    } else {
      if (!funcSet.has(handler)) {
        funcSet.add(handler);
      }
      return RESULT_CODE.SUCCESS;
    }
  }

  /**
   * 关闭接收数据事件
   * @param dport
   * @param handler
   * @returns
   */
  async offData(dport: string, handler?: Bnrtc2.Bnrtc2DataHandler): Promise<RESULT_CODE> {
    const functions = this._dport2FunctionsMap.get(dport);
    if (!functions || functions.size <= 0) {
      return RESULT_CODE.FAILED;
    }

    if (handler) {
      functions.delete(handler);
      if (functions.size > 0) {
        return RESULT_CODE.SUCCESS;
      }
    }

    this._dport2FunctionsMap.delete(dport);
    return this._sendUnbindDportMsg(dport);
  }

  /**
   * 监听到消息后运行handler
   * @param data
   */
  private async _runHandler(data: Uint8Array) {
    let msg: Uint8Array | undefined = undefined
    try {
      msg = await eventDataReader(data);
      /*
      * Data | Sync
      * ---------------------------------------------------------------------------
      * |  u8   |  str  |   str   |   str   |    str    |   str    | bool | bytes |
      * ---------------------------------------------------------------------------
      * | type  | dport | srcAddr | dstAddr | srcDevid  | dstDevid | sync |  data |
      * ---------------------------------------------------------------------------
      **/
      const resBuf = Bnrtc2Buffer.from(msg as Uint8Array);
      const type = resBuf.pullU8();
      const dport = resBuf.pullStr();
      const src = resBuf.pullStr();
      const dst = resBuf.pullStr();
      const devid = resBuf.pullStr();
      resBuf.pullStr(); // discard dstDevid
      const isSync = resBuf.pullBool();
      if (type === MESSAGE_TYPE.ACK) {
        const msgId = resBuf.pullU32();
        const resultCode = resBuf.pullU8();
        const task = this._msg2TaskMap.tryGet(msgId);
        if (task) {
          if (resultCode === RESULT_CODE.SUCCESS) {
            task.resolve(RESULT_CODE.SUCCESS);
          } else {
            task.reject(resultCode as RESULT_CODE);
          }
        }
      } else if (type === MESSAGE_TYPE.DATA || type === MESSAGE_TYPE.SYNC) {
        const functions = this._dport2FunctionsMap.get(dport);
        if (functions) {
          for (const func of functions) {
            func({ address: src, dport, data: resBuf.data(), dst, devid, isSync });
          }
        }
      }
    } catch {
    }
  }

  private async _sendBindDportMsg(dport: string) {
    return this._send(MESSAGE_TYPE.BINDDPORT, dport, "", "", "", undefined);
  }

  private async _sendUnbindDportMsg(dport: string) {
    return this._send(MESSAGE_TYPE.UNBINDDPORT, dport, "", "", "", undefined);
  }

  /**
   * 关闭client
   * @param code
   * @param reason
   * @returns
   */
  private _close(code?: number, reason?: string): boolean {
    this._dport2FunctionsMap.clear();
    this._msg2TaskMap.forEach((task) => {
      task.reject(RESULT_CODE.FAILED)
    })
    this._msg2TaskMap.clear()
    this._api.disconnectChannel(this._channel, code, reason);
    return true;
  }
}
