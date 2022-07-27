import "@bfchain/bnrtc2-client-typings";



export interface IBnrtcController {
  getServiceInfo(name: string): Promise<Bnrtc2.Bnrtc2ServiceInfo | undefined>;
}

export interface IBnrtcClient {
  send(address: string, dport: string, data: Bnrtc2.Bnrtc2Buffer | Uint8Array, devid?: string, src?: string): void;
  onData(dport: string, handler: Bnrtc2.Bnrtc2DataHandler): void;
  offData(dport: string, handler: Bnrtc2.Bnrtc2DataHandler): void;
  readonly onOpen: import('@bfcs/util-evt').StatefulAttacher<boolean>;
  readonly controller: IBnrtcController;
}

export type MethodType = "GET" | "POST" | "PUT" | "DELETE" | "PATCH" | "HEAD";
export type DwebRequestMsg = {
  reqId: number;
  path: string;
  method: MethodType;
  header: { [key in string]: string };
  postForm?: { [key in string]: string };
};
export type ResponseHeader = { [key in string]: string };
export type ResponseMsg = {
  reqId: number;
  statusCode: number;
  data: Uint8Array;
  header: ResponseHeader;
};
export type AssertType =
  | "json"
  | "text"
  | "html"
  | "xml"
  | "gif"
  | "png"
  | "jpeg"
  | "octetStream"
  | "formData"
  | "formUrlencoded"
  | "javascript"
  | "ico"
  | "svg"
  | "css";
export type IDwebHandler = (
  req: import("./dwebRequest").DwebRequest,
  res: import("./dwebResponse").DwebResponse,
  next: () => void
) => Promise<void> | void;

export type IDwebLns = {
  host: string;
  pathname?: string;
  address: string;
};

export type pageDownloadEvData = {
  dPort: string;
  url: string;
  type: "pageDownload";
};

export type DownloadFileInfo = {
  url: string;
  start?: number | undefined;
};
