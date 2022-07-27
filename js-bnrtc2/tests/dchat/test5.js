"use strict";
exports.__esModule = true;
var bnrtc2_dchat_node_1 = require("@bfchain/bnrtc2-dchat-node");
var bnrtc2_client_1 = require("@bfchain/bnrtc2-client");
var bnrtc2_dchat_typings_1 = require("@bfchain/bnrtc2-dchat-typings");
var util_encoding_utf8_1 = require("@bfchain/util-encoding-utf8");
var bnrtc2_buffer_1 = require("@bfchain/bnrtc2-buffer");
var DCHAT_DPORT = bnrtc2_dchat_typings_1.DCHAT_DPORT_PREFIX + "bfchain";
function logIt() {
    var args = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        args[_i] = arguments[_i];
    }
    console.log(args);
}
var controller = new bnrtc2_client_1.Bnrtc2Controller("127.0.0.1", 19888);
var localaddress = "C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D";
var remoteAddress = "3jui8ko762DHguaNcfoHgJkWvCWsUo4ok";
controller.bindAddress(localaddress);
function messageHandler(address, dport, data) {
    var msg = (0, util_encoding_utf8_1.decodeBinaryToUTF8)(data);
    console.log("%s: %s",address, msg);
    return true;
}
var dchat = new bnrtc2_dchat_node_1.Dchat("127.0.0.1", 19888);
dchat.onMessage(DCHAT_DPORT, messageHandler);
var readline = require('readline');
var rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout
});
rl.on('line', function (str) {
    // str即为输入的内容
    if (str === 'close') {
        // 关闭逐行读取流 会触发关闭事件
        rl.close();
    }
    var info = str.split(" ", 2);
    if (info[0] == "--local") {
        localaddress = info[1];
        controller.bindAddress(localaddress);
    }
    else if (info[0] == "--peer") {
        remoteAddress = info[1];
    }
    else {
        var buf = bnrtc2_buffer_1.Bnrtc2Buffer.create(100);
        buf.pushStr(str);
        dchat.send(remoteAddress, DCHAT_DPORT, buf.data());
    }
});
rl.on("close", function () {
    process.exit(0);
});
