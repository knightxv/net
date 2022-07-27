import { DwebServer } from "@bfchain/bnrtc2-dweb";
const dwebServer = new DwebServer();
dwebServer.get("/", (req, res) => {
  res.setAssertType("html");
  res.end('<div style="color: red">node js web server</div>');
});
dwebServer.listen("dweb");
