import type { IBnrtcClient, IDwebHandler, IDwebLns } from "./types";
import { hextoString } from "./util/index";
import { Bnrtc2Client } from "@bfchain/bnrtc2-client";
import { DwebResponse } from "./dwebResponse";
import { DwebRequest } from "./dwebRequest";
import { encodeUTF8ToBinary } from "@bfchain/util-encoding-utf8";
import { bnrtc2Global } from "@bfchain/bnrtc2-client-api";
import { DownLoadManger } from "./downLoadManger";
const MAX_SECOND_LEVEL_HOST_LENGTH = 62;

type IUseParams = {
  PathHander: [string, IDwebHandler];
  AllHander: [IDwebHandler];
};

export class DwebServer {
  constructor(public bnrtc2Client: IBnrtcClient = new Bnrtc2Client()) {}
  pluginHandlerList: IDwebHandler[] = [];
  afterPluginHandlerList: IDwebHandler[] = [];
  pathHandlerMap: Map<
    /*pathName*/ string,
    Map</*method*/ string, IDwebHandler[]>
  > = new Map();
  regexpHandlerMap: Map</*method*/ string, [RegExp, IDwebHandler][]> =
    new Map();
  use(...arg: IUseParams["AllHander"] | IUseParams["PathHander"]) {
    if (typeof arg[0] === "string") {
      arg = arg as IUseParams["PathHander"];
      const reg = new RegExp(`^\\${arg[0]}((\\/.)+)?`);
      const handler = arg[1];
      const filterHandler: IDwebHandler = (req, res, next) => {
        if (reg.test(req.path)) {
          req.useData["relativePath"] = req.path.replace(reg, "");
          handler(req, res, next);
          return;
        }
        next();
      };
      this.pluginHandlerList.push(filterHandler);
    } else {
      arg = arg as IUseParams["AllHander"];
      this.pluginHandlerList.push(arg[0]);
    }
  }
  /** 滞后使用插件 */
  afterUse(...arg: IUseParams["AllHander"] | IUseParams["PathHander"]) {
    if (typeof arg[0] === "string") {
      arg = arg as IUseParams["PathHander"];
      const reg = new RegExp(`^\\${arg[0]}((\\/.)+)?`);
      const handler = arg[1];
      const filterHandler: IDwebHandler = (req, res, next) => {
        if (reg.test(req.path)) {
          req.path = req.path.replace(reg, "");
          handler(req, res, next);
          return;
        }
        next();
      };
      this.afterPluginHandlerList.push(filterHandler);
    } else {
      arg = arg as IUseParams["AllHander"];
      this.afterPluginHandlerList.push(arg[0]);
    }
  }
  all(pathName: string | RegExp, handler: IDwebHandler) {
    const filterHandler: IDwebHandler = (req, res, next) => {
      if (typeof pathName === "string") {
        if (req.path === pathName) {
          handler(req, res, next);
          return;
        }
      } else {
        if (pathName.test(req.path)) {
          handler(req, res, next);
          return;
        }
      }
      next();
    };
    this.pluginHandlerList.push(filterHandler);
  }
  get(pathName: string | RegExp, handler: IDwebHandler) {
    if (typeof pathName === "string") {
      this.__setPathHandler(pathName, "GET", handler);
    } else {
      this.__setRegexpHandler(pathName, "GET", handler);
    }
  }
  post(pathName: string, handler: IDwebHandler) {
    if (typeof pathName === "string") {
      this.__setPathHandler(pathName, "POST", handler);
    } else {
      this.__setRegexpHandler(pathName, "POST", handler);
    }
  }
  put(pathName: string, handler: IDwebHandler) {
    if (typeof pathName === "string") {
      this.__setPathHandler(pathName, "PUT", handler);
    } else {
      this.__setRegexpHandler(pathName, "PUT", handler);
    }
  }
  patch(pathName: string, handler: IDwebHandler) {
    if (typeof pathName === "string") {
      this.__setPathHandler(pathName, "PATCH", handler);
    } else {
      this.__setRegexpHandler(pathName, "PATCH", handler);
    }
  }
  delete(pathName: string, handler: IDwebHandler) {
    if (typeof pathName === "string") {
      this.__setPathHandler(pathName, "DELETE", handler);
    } else {
      this.__setRegexpHandler(pathName, "DELETE", handler);
    }
  }
  __setPathHandler(pathName: string, method: string, handler: IDwebHandler) {
    let methodHandlerMap = this.pathHandlerMap.get(pathName);
    if (methodHandlerMap == undefined) {
      methodHandlerMap = new Map([[method, [handler]]]);
      this.pathHandlerMap.set(pathName, methodHandlerMap);
      return;
    }
    const handerList = methodHandlerMap.get(method);
    if (handerList == undefined) {
      throw new ReferenceError("is in possible to undefined");
    }
    handerList.push(handler);
  }
  __setRegexpHandler(regexp: RegExp, method: string, handler: IDwebHandler) {
    let regExpHandlerList = this.regexpHandlerMap.get(method);
    if (regExpHandlerList == undefined) {
      regExpHandlerList = [[regexp, handler]];
      this.regexpHandlerMap.set(method, regExpHandlerList);
      return;
    }
    regExpHandlerList.push([regexp, handler]);
  }
  dPort?: string;
  listen(dPort: string = "dweb") {
    if (this.dPort)
      throw new ReferenceError("have listen dport::" + this.dPort);
    this.dPort = dPort;
    this.bnrtc2Client.onOpen.attach(() => {
        this.bnrtc2Client.onData(dPort, this.messageHandler);
    });
  }
  close() {
    if (!this.dPort) return;
    this.bnrtc2Client.offData(this.dPort, this.messageHandler);
    this.dPort = undefined;
  }
  messageHandler = (evt: Bnrtc2.Bnrtc2Data) => {
    try {
      const dwebRequest = new DwebRequest(evt.data);
      const dwebResponse = new DwebResponse(
        this.bnrtc2Client,
        dwebRequest.reqId,
        evt.address,
        evt.dport,
        evt.devid
      );
      this.serverHandler(dwebRequest, dwebResponse);
    } catch (err) {
      /// 解析数据失败
    }
  };
  async serverHandler(req: DwebRequest, res: DwebResponse) {
    try {
      /// 先运行插件
      {
        const isEnd = await this.runPluginHandler(
          this.pluginHandlerList,
          req,
          res
        );
        if (isEnd) {
          return;
        }
      }
      /// 路径匹配
      {
        const isEnd = await this.runPathHandler(req, res);
        if (isEnd) {
          return;
        }
      }
      /// 最后运行滞后插件
      {
        const isEnd = await this.runPluginHandler(
          this.afterPluginHandlerList,
          req,
          res
        );
        if (isEnd) {
          return;
        }
      }
    } catch (err) {
    } finally {
      if (res.isSend) {
        return;
      }
      res.end();
    }
  }
  /** 匹配path运行对应handler，范围从大到小 */
  async runPathHandler(req: DwebRequest, res: DwebResponse): Promise<boolean> {
    const regExpHandlerList = this.regexpHandlerMap.get(req.method);
    if (regExpHandlerList != undefined) {
      for (const [regexp, handler] of regExpHandlerList) {
        if (regexp.test(req.path)) {
          let isEnd = true;
          await new Promise<void>(async (resolve) => {
            await handler(req, res, () => {
              isEnd = false;
            });
            resolve();
          });
          if (isEnd) {
            return true;
          }
        }
      }
    }
    const matchList = req.path.match(/^\/([\/\w]+)?/);
    let pathName = req.path;
    if (matchList) {
      pathName = matchList[0];
    }
    const pathHandlerMap = this.pathHandlerMap.get(pathName);
    if (pathHandlerMap != undefined) {
      const handlerList = pathHandlerMap.get(req.method);
      if (handlerList != undefined) {
        for (const handler of handlerList) {
          let isEnd = true;
          await new Promise<void>(async (resolve) => {
            await handler(req, res, () => {
              isEnd = false;
            });
            resolve();
          });
          if (isEnd) {
            return true;
          }
        }
      }
    }
    return false;
  }
  async runPluginHandler(
    pluginHandlerList: IDwebHandler[],
    req: DwebRequest,
    res: DwebResponse
  ) {
    return new Promise<boolean>(async (resolve) => {
      let isEnd = true;
      for (let i = 0; i <= pluginHandlerList.length - 1; i++) {
        const pluginHandler = pluginHandlerList[i];
        await pluginHandler(req, res, () => {
          isEnd = false;
        });
        if (isEnd) {
          resolve(true);
          return;
        }
      }
      resolve(false);
    });
  }
  static(root: string): IDwebHandler {
    return (req, res, next) => {};
  }
  private _dwebPort: number | undefined;
  async getDwebPort() {
    if (this._dwebPort) {
      return this._dwebPort;
    }
    const confRes = await this.bnrtc2Client.controller.getServiceInfo("dweb");
    if (confRes === undefined|| isNaN(confRes.port)) {
      throw new ReferenceError("can not get get dwebPort");
    }
    return (this._dwebPort = confRes.port) as number;
  }
  async dwebResolve(lns: IDwebLns) {
    const url = `${lns.address}:${lns.host}`;
    const hexAddress = hextoString(encodeUTF8ToBinary(url));
    const dwebPort = await this.getDwebPort();
    let addressSplitOfsset = 0;
    const addressSplitList: string[] = [];
    do {
      addressSplitList.push(
        hexAddress.slice(
          addressSplitOfsset,
          addressSplitOfsset + MAX_SECOND_LEVEL_HOST_LENGTH
        )
      );
      addressSplitOfsset += MAX_SECOND_LEVEL_HOST_LENGTH;
    } while (addressSplitOfsset < hexAddress.length);
    return `http://${addressSplitList.join(".")}.localhost:${dwebPort}${
      lns.pathname || ""
    }`;
  }
  async customNotFoundPage(data: Uint8Array) {
    const dwebPort = await this.getDwebPort();
    const postUrl = `http://localhost:${dwebPort}/customNotFoundPage`;
    return bnrtc2Global.fetch(postUrl, {
      method: "POST",
      body: data,
    });
  }
  async getDownLoadManger(dPort: string = "*") {
    const dwebPort = await this.getDwebPort();
    return new DownLoadManger(dPort, dwebPort);
  }
}
