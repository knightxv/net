import test from "ava";

import type { ResponseMsg } from "../src";
import { encodeDwebResponse, decodeDwebResponse, DwebRequest } from "../src";

test("encodeDwebResponse decodeDwebResponse", async (t) => {
  const res3: ResponseMsg = {
    reqId: 233,
    statusCode: 200,
    header: {
      contentType: "text/html",
    },
    data: new Uint8Array([1, 2, 3]),
  };
  const bytes2 = encodeDwebResponse(res3);
  const res4 = decodeDwebResponse(bytes2);
  t.deepEqual(res3, res4);
});
test("parse DwebResponse", async (t) => {
  const parseQuery = DwebRequest.prototype.parseQuery;
  {
    const query = parseQuery("/abc?a=1");
    t.deepEqual(query, { a: "1" });
  }
  {
    const query = parseQuery("/abc?a=1&b=sdf");
    t.deepEqual(query, { a: "1", b: "sdf" });
  }
});
