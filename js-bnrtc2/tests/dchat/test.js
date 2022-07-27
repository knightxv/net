"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __generator = (this && this.__generator) || function (thisArg, body) {
    var _ = { label: 0, sent: function() { if (t[0] & 1) throw t[1]; return t[1]; }, trys: [], ops: [] }, f, y, t, g;
    return g = { next: verb(0), "throw": verb(1), "return": verb(2) }, typeof Symbol === "function" && (g[Symbol.iterator] = function() { return this; }), g;
    function verb(n) { return function (v) { return step([n, v]); }; }
    function step(op) {
        if (f) throw new TypeError("Generator is already executing.");
        while (_) try {
            if (f = 1, y && (t = op[0] & 2 ? y["return"] : op[0] ? y["throw"] || ((t = y["return"]) && t.call(y), 0) : y.next) && !(t = t.call(y, op[1])).done) return t;
            if (y = 0, t) op = [op[0] & 2, t.value];
            switch (op[0]) {
                case 0: case 1: t = op; break;
                case 4: _.label++; return { value: op[1], done: false };
                case 5: _.label++; y = op[1]; op = [0]; continue;
                case 7: op = _.ops.pop(); _.trys.pop(); continue;
                default:
                    if (!(t = _.trys, t = t.length > 0 && t[t.length - 1]) && (op[0] === 6 || op[0] === 2)) { _ = 0; continue; }
                    if (op[0] === 3 && (!t || (op[1] > t[0] && op[1] < t[3]))) { _.label = op[1]; break; }
                    if (op[0] === 6 && _.label < t[1]) { _.label = t[1]; t = op; break; }
                    if (t && _.label < t[2]) { _.label = t[2]; _.ops.push(op); break; }
                    if (t[2]) _.ops.pop();
                    _.trys.pop(); continue;
            }
            op = body.call(thisArg, _);
        } catch (e) { op = [6, e]; y = 0; } finally { f = t = 0; }
        if (op[0] & 5) throw op[1]; return { value: op[0] ? op[1] : void 0, done: true };
    }
};
exports.__esModule = true;
var bnrtc2_dchat_node_1 = require("@bfchain/bnrtc2-dchat-node");
var bnrtc2_dchat_typings_1 = require("@bfchain/bnrtc2-dchat-typings");
var bnrtc2_buffer_1 = require("@bfchain/bnrtc2-buffer");
var http = require("http");
var bnrtc2_client_1 = require("@bfchain/bnrtc2-client");
var DCHAT_DPORT = bnrtc2_dchat_typings_1.DCHAT_DPORT_PREFIX + "bfchain";
var configs = [
    {
        name: "node1",
        localAddress: "C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D",
        remoteAddress: [
            "3jui8ko762DHguaNcfoHgJkWvCWsUo4ok",
            "QFbkas6RKsCeGTUQRUESGmWvEAZN8yNCE",
        ],
        httpPort: 11111,
        bnrtcPort: 19888
    },
    {
        name: "node2",
        localAddress: "3jui8ko762DHguaNcfoHgJkWvCWsUo4ok",
        remoteAddress: [
            "C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D",
            "QFbkas6RKsCeGTUQRUESGmWvEAZN8yNCE",
        ],
        httpPort: 11112,
        bnrtcPort: 19999
    },
    {
        name: "node2-1",
        localAddress: "3jui8ko762DHguaNcfoHgJkWvCWsUo4ok",
        remoteAddress: ["C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D", "QFbkas6RKsCeGTUQRUESGmWvEAZN8yNCE"],
        httpPort: 11113,
        bnrtcPort: 20000
    }
];
var loginConfig = {
    name: "node3",
    localAddress: "QFbkas6RKsCeGTUQRUESGmWvEAZN8yNCE",
    remoteAddress: ["3jui8ko762DHguaNcfoHgJkWvCWsUo4ok", "C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D"],
    httpPort: 11114,
    bnrtcPort: 20001
};
function logIt() {
    var args = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        args[_i] = arguments[_i];
    }
    console.log(args);
}
function startDchatService(config) {
    return __awaiter(this, void 0, void 0, function () {
        var dchat, server;
        var _this = this;
        return __generator(this, function (_a) {
            dchat = new bnrtc2_dchat_node_1.Dchat("127.0.0.1", config.bnrtcPort);
            dchat.onMessage(DCHAT_DPORT, function (address, dport, data, devid, src, isSync) {
                var msg = bnrtc2_buffer_1.Bnrtc2Buffer.from(data).pullStr();
                if (isSync) {
                    logIt(config.name, config.localAddress, " got Sync message: ", address, dport, devid, src, msg);
                }
                else {
                    logIt(config.name, config.localAddress, " got Send message: ", address, dport, devid, src, msg);
                }
                return true;
            });
            dchat.onLoginStatusChange(function (addresses, state) {
                logIt(config.name, config.localAddress, "receive login status: ", state, addresses);
            });
            dchat.onStateChange(function (state) {
                return __awaiter(this, void 0, void 0, function () {
                    return __generator(this, function (_a) {
                        switch (_a.label) {
                            case 0:
                                logIt(config.name, config.localAddress, "receive ready status: ", state);
                                if (!(state == 1 /* OPEN */)) return [3 /*break*/, 3];
                                return [4 /*yield*/, dchat.login(config.localAddress)];
                            case 1:
                                _a.sent();
                                return [4 /*yield*/, dchat.addFriends(config.localAddress, config.remoteAddress)];
                            case 2:
                                _a.sent();
                                _a.label = 3;
                            case 3: return [2 /*return*/];
                        }
                    });
                });
            });
            console.log("start server", config.localAddress, config.remoteAddress, config.httpPort, config.bnrtcPort);
            server = http.createServer(function (req, res) { return __awaiter(_this, void 0, void 0, function () {
                var url, address, res_1, message_1, buf, _loop_1, _i, _a, address;
                return __generator(this, function (_b) {
                    switch (_b.label) {
                        case 0:
                            //设置允许跨域的域名，*代表允许任意域名跨域
                            res.setHeader("Access-Control-Allow-Origin", "*");
                            //跨域允许的header类型
                            res.setHeader("Access-Control-Allow-Headers", "Content-type,Content-Length,Authorization,Accept,X-Requested-Width");
                            //跨域允许的请求方式
                            res.setHeader("Access-Control-Allow-Methods", "PUT,POST,GET,DELETE,OPTIONS");
                            //设置响应头信息
                            res.setHeader("X-Powered-By", " 3.2.1");
                            if (!(req.method == "GET")) return [3 /*break*/, 7];
                            url = new URL(req.url || "", "http://127.0.0.1");
                            if (!(url.pathname === "/connect")) return [3 /*break*/, 2];
                            address = url.searchParams.get("address");
                            return [4 /*yield*/, dchat.connect(address)];
                        case 1:
                            res_1 = _b.sent();
                            logIt("connect remote address: ", config.remoteAddress, res_1);
                            return [3 /*break*/, 6];
                        case 2:
                            if (!(url.pathname === "/message")) return [3 /*break*/, 6];
                            message_1 = url.searchParams.get("message") || "";
                            logIt("send message : ", message_1);
                            buf = bnrtc2_buffer_1.Bnrtc2Buffer.create(100);
                            buf.pushStr(message_1);
                            _loop_1 = function (address) {
                                return __generator(this, function (_c) {
                                    switch (_c.label) {
                                        case 0: return [4 /*yield*/, dchat
                                                .sendOne(address, DCHAT_DPORT, buf.data())
                                                .then(function (r) {
                                                logIt("send One data Code: ", r);
                                                if (r == bnrtc2_dchat_typings_1.MessageState.Success) {
                                                    logIt("send One data: ", message_1, address, "ok");
                                                }
                                                else {
                                                    logIt("send One data ", message_1, config.remoteAddress, "failed", r);
                                                }
                                            })];
                                        case 1:
                                            _c.sent();
                                            return [2 /*return*/];
                                    }
                                });
                            };
                            _i = 0, _a = config.remoteAddress;
                            _b.label = 3;
                        case 3:
                            if (!(_i < _a.length)) return [3 /*break*/, 6];
                            address = _a[_i];
                            return [5 /*yield**/, _loop_1(address)];
                        case 4:
                            _b.sent();
                            _b.label = 5;
                        case 5:
                            _i++;
                            return [3 /*break*/, 3];
                        case 6: return [2 /*return*/, res.end()];
                        case 7: return [2 /*return*/];
                    }
                });
            }); });
            server.listen(config.httpPort);
            return [2 /*return*/, {
                    dchat: dchat,
                    server: server
                }];
        });
    });
}
function startControlService() {
    var _this = this;
    var loginDchat = undefined;
    var loginServer = undefined;
    var controller = new bnrtc2_client_1.Bnrtc2Controller("127.0.0.1", loginConfig.bnrtcPort);
    http.createServer(function (req, res) { return __awaiter(_this, void 0, void 0, function () {
        var url, _a, dchat, server, address, online, service, info;
        return __generator(this, function (_b) {
            switch (_b.label) {
                case 0:
                    //设置允许跨域的域名，*代表允许任意域名跨域
                    res.setHeader("Access-Control-Allow-Origin", "*");
                    //跨域允许的header类型
                    res.setHeader("Access-Control-Allow-Headers", "Content-type,Content-Length,Authorization,Accept,X-Requested-Width");
                    //跨域允许的请求方式
                    res.setHeader("Access-Control-Allow-Methods", "PUT,POST,GET,DELETE,OPTIONS");
                    //设置响应头信息
                    res.setHeader("X-Powered-By", " 3.2.1");
                    if (!(req.method == "GET")) return [3 /*break*/, 11];
                    url = new URL(req.url || "", "http://127.0.0.1");
                    if (!(url.pathname === "/login")) return [3 /*break*/, 2];
                    logIt("login : ");
                    return [4 /*yield*/, startDchatService(loginConfig)];
                case 1:
                    _a = _b.sent(), dchat = _a.dchat, server = _a.server;
                    loginDchat = dchat;
                    loginServer = server;
                    res.end("login success");
                    return [3 /*break*/, 11];
                case 2:
                    if (!(url.pathname === "/logout")) return [3 /*break*/, 6];
                    logIt("logout: ");
                    return [4 /*yield*/, (loginDchat === null || loginDchat === void 0 ? void 0 : loginDchat.logout(loginConfig.localAddress))];
                case 3:
                    _b.sent();
                    return [4 /*yield*/, (loginDchat === null || loginDchat === void 0 ? void 0 : loginDchat.close())];
                case 4:
                    _b.sent();
                    return [4 /*yield*/, (loginServer === null || loginServer === void 0 ? void 0 : loginServer.close())];
                case 5:
                    _b.sent();
                    res.end("logout success");
                    return [3 /*break*/, 11];
                case 6:
                    if (!(url.pathname === "/isonline")) return [3 /*break*/, 8];
                    address = url.searchParams.get("address");
                    return [4 /*yield*/, controller.isOnline(address)];
                case 7:
                    online = _b.sent();
                    logIt("isOnline: ", address, online);
                    res.end(online);
                    return [3 /*break*/, 11];
                case 8:
                    if (!(url.pathname === "/info")) return [3 /*break*/, 10];
                    service = url.searchParams.get("service");
                    return [4 /*yield*/, controller.getServiceInfo(service)];
                case 9:
                    info = _b.sent();
                    logIt("serviceInfo: ", service, info);
                    res.end(JSON.stringify(info));
                    return [3 /*break*/, 11];
                case 10:
                    res.end("error");
                    _b.label = 11;
                case 11: return [2 /*return*/];
            }
        });
    }); }).listen(10000);
    logIt("start control service: ", 10000);
}
startControlService();
for (var _i = 0, configs_1 = configs; _i < configs_1.length; _i++) {
    var config = configs_1[_i];
    startDchatService(config);
}
