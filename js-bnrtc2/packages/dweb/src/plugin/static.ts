import { mediaTypeMap } from "./mediaTypeMap";
import type { IDwebHandler } from "../types";

type StaticFileStat = {
  /** 文件大小 */
  size: number;
};

export type StaticFileReader = {
  readFile(
    root: string,
    relativeFile: string,
    start?: number,
    end?: number
  ): Promise<Uint8Array>;
  stat(root: string, relativeFile: string): Promise<StaticFileStat>;
};
export type StaticOption = {
  maxAge: number;
};

export function serverStatic(
  fileReader: StaticFileReader,
  root: string,
  options?: StaticOption
): IDwebHandler {
  if (options == undefined) {
    /// 默认配置1年过期（单位秒）
    options = { maxAge: 31536000 };
  }
  const opt = options as StaticOption;
  return async (req, res, next) => {
    let reqFilePath = req.useData["relativePath"] || req.path;
    if (!reqFilePath) {
      next();
      return;
    }
    if (reqFilePath[reqFilePath.length - 1] === "/") {
      reqFilePath += "index.html";
    }
    const fileTypeReg = /\.(\w+)(\?.+)?$/;
    const fileMatch = reqFilePath.match(fileTypeReg);
    if (fileMatch) {
      const fileType = fileMatch[1];
      const supportContentType = mediaTypeMap[`.${fileType}`];
      if (supportContentType) {
        if (fileType != "html") {
          res.setHeader("Cache-Control", `max-age=${opt.maxAge}`);
        }
        res.setHeader("Content-Type", supportContentType);
      }
    }
    try {
      const { size } = await fileReader.stat(root, reqFilePath);
      res.setHeader("Content-Length", "" + size);
      res.setHeader("Accept-Ranges", "bytes");
      if (req.method === "HEAD") {
        res.end("");
        return;
      }
      const range = req.header["Range"]?.[0];
      if (range) {
        const fileSize = size; /// bytelength
        const rangeList = range.replace("bytes=", "").split("-");
        const bufferSize = 1024 * 1024 * 1; //每次传输的字节数，或设置未为fileSize，让客户端尽可能长的获取字节数
        let startPos = parseInt(rangeList[0], 10);
        let endPos = startPos + bufferSize;
        if (endPos >= fileSize) {
          endPos = fileSize;
        }
        const ContentLength = endPos - startPos;
        res.setHeader("Accept-Ranges", "bytes");
        res.setHeader(
          "Content-Range",
          "bytes " + startPos + "-" + endPos + "/" + fileSize
        );
        res.setHeader("Content-Length", "" + ContentLength);
        res.status(206);
        if (endPos != fileSize - 1) {
          res.setHeader("Content-Type", "application/octet-stream");
        }
        const data = await fileReader.readFile(
          root,
          reqFilePath,
          startPos,
          endPos
        );
        res.end(data);
        return;
      }
      const data = await fileReader.readFile(root, reqFilePath);
      res.end(data);
    } catch (error) {
      next();
    }
  };
}
