import type { AssertType } from "./types";

export const assertContentTypeMap: { [keyof in AssertType]: string } = {
  html: "text/html",
  text: "text/plain",
  xml: "text/xml",
  gif: "image/gif",
  png: "image/png",
  jpeg: "image/jpeg ",
  svg: "text/xml",
  json: "application/json",
  octetStream: "application/octet-stream",
  formData: "multipart/form-data",
  formUrlencoded: "application/x-www-form-urlencoded",
  javascript: "text/javascript",
  css: "text/css",
  ico: "image/x-icon",
};
