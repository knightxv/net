"use strict";
exports.__esModule = true;
var bnrtc2_dchat_node_1 = require("@bfchain/bnrtc2-dchat-node");
var bnrtc2_dchat_typings_1 = require("@bfchain/bnrtc2-dchat-typings");
var bnrtc2_buffer_1 = require("@bfchain/bnrtc2-buffer");
var http = require("http");
var DCHAT_DPORT = bnrtc2_dchat_typings_1.DCHAT_DPORT_PREFIX + "bfchain";
function logIt() {
    var args = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        args[_i] = arguments[_i];
    }
    console.log(args);
}
function messageHandler(address, dport, data) {
    var buf = bnrtc2_buffer_1.Bnrtc2Buffer.from(data);
    var msg = buf.pullStr()
    logIt("dchat1 got message: ", address, dport, msg);
    return true;
}
var localaddress = "C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D";
var remoteAddress = "3jui8ko762DHguaNcfoHgJkWvCWsUo4ok";
var dchat = new bnrtc2_dchat_node_1.Dchat("127.0.0.1", 19888);
dchat.onMessage(DCHAT_DPORT, messageHandler);
http.createServer(function (req, res) {
    //设置允许跨域的域名，*代表允许任意域名跨域
    res.setHeader("Access-Control-Allow-Origin", "*");
    //跨域允许的header类型
    res.setHeader("Access-Control-Allow-Headers", "Content-type,Content-Length,Authorization,Accept,X-Requested-Width");
    //跨域允许的请求方式
    res.setHeader("Access-Control-Allow-Methods", "PUT,POST,GET,DELETE,OPTIONS");
    //设置响应头信息
    res.setHeader("X-Powered-By", ' 3.2.1');
    //让options请求快速返回
    if (req.method == "GET") {
        var url = new URL(req.url || "", "http://127.0.0.1");
        if (url.pathname === "/localaddress") {
            localaddress = url.searchParams.get("address") || "";
            logIt("set local address: ", localaddress);
        }
        else if (url.pathname === "/remoteAddress") {
            remoteAddress = url.searchParams.get("address") || "";
            logIt("set remote address: ", remoteAddress);
        }
        else if (url.pathname === "/message") {
            var message_1 = url.searchParams.get("message") || "";
            var buf = bnrtc2_buffer_1.Bnrtc2Buffer.create(100);
            buf.pushStr(message_1);
            dchat.send(remoteAddress, DCHAT_DPORT, buf.data()).then(function (r) {
                if (r == bnrtc2_dchat_typings_1.MessageState.Success) {
                    logIt("send data: ", message_1, remoteAddress, "ok");
                }
                else {
                    logIt("send data: ", message_1, remoteAddress, "failed", bnrtc2_dchat_typings_1.MessageState[r]);
                }
            });
        }
        return res.end();
    }
}).listen(19555);
