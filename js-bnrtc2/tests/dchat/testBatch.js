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
var DCHAT_DPORT = bnrtc2_dchat_typings_1.DCHAT_DPORT_PREFIX + "bfchainBatch";
var num = 20;
var startPort = 10000;
var peers = [];
var Peer = /** @class */ (function () {
    function Peer(address, friends, port) {
        this.friends = [];
        this.receiveCnt = 0;
        this.sendCnt = 0;
        this.sendFailedCnt = 0;
        this.address = address;
        this.port = port;
        this.friends = friends;
        this.dchat = new bnrtc2_dchat_node_1.Dchat("127.0.0.1", this.port);
    }
    Peer.prototype.init = function () {
        return __awaiter(this, void 0, void 0, function () {
            var self;
            return __generator(this, function (_a) {
                self = this;
                this.dchat.onMessage(DCHAT_DPORT, function (address, dport, data) {
                    self.receiveCnt++;
                    return true;
                });
                this.dchat.onStateChange(function (state) {
                    return __awaiter(this, void 0, void 0, function () {
                        return __generator(this, function (_a) {
                            switch (_a.label) {
                                case 0:
                                    if (!(state == 1 /* OPEN */)) return [3 /*break*/, 3];
                                    return [4 /*yield*/, self.dchat.login(self.address)];
                                case 1:
                                    _a.sent();
                                    return [4 /*yield*/, self.dchat.addFriends(self.address, self.friends)];
                                case 2:
                                    _a.sent();
                                    _a.label = 3;
                                case 3: return [2 /*return*/];
                            }
                        });
                    });
                });
                return [2 /*return*/];
            });
        });
    };
    Peer.prototype.send = function (msg) {
        return __awaiter(this, void 0, void 0, function () {
            var pererAddress, buf;
            var _this = this;
            return __generator(this, function (_a) {
                switch (_a.label) {
                    case 0:
                        pererAddress = peers[1 + Math.floor(Math.random() * (num - 1))].address;
                        if (pererAddress === this.address) {
                            return [2 /*return*/];
                        }
                        buf = bnrtc2_buffer_1.Bnrtc2Buffer.create(0);
                        buf.putStr(msg);
                        return [4 /*yield*/, this.dchat.send(pererAddress, DCHAT_DPORT, buf.data()).then(function (resCode) {
                                if (resCode == bnrtc2_dchat_typings_1.MessageState.Success) {
                                    _this.sendCnt++;
                                    console.log("%s send msg '%s' to  %s ok ", _this.address, msg, pererAddress);
                                }
                                else {
                                    console.log("%s send msg '%s' to  %s err %s ", _this.address, msg, pererAddress, resCode);
                                    _this.sendFailedCnt++;
                                }
                            })];
                    case 1:
                        _a.sent();
                        return [2 /*return*/];
                }
            });
        });
    };
    return Peer;
}());
for (var i = 0; i < num; i++) {
    var address = "batchTest" + String([i]);
    var friends = [];
    for (var j = 0; j < num; j++) {
        if (i != j) {
            friends.push("batchTest" + String([j]));
        }
    }
    peers.push(new Peer(address, friends, startPort + i));
}
(function run() {
    return __awaiter(this, void 0, void 0, function () {
        var j, i, peer, i, peer;
        return __generator(this, function (_a) {
            switch (_a.label) {
                case 0:
                    j = 0;
                    i = 1;
                    _a.label = 1;
                case 1:
                    if (!(i < peers.length)) return [3 /*break*/, 4];
                    peer = peers[i];
                    return [4 /*yield*/, peer.init()];
                case 2:
                    _a.sent();
                    _a.label = 3;
                case 3:
                    i++;
                    return [3 /*break*/, 1];
                case 4:
                    for (i = 1; i < peers.length; i++) {
                        peer = peers[i];
                        setInterval(function (p) {
                            p.send("hello-" + (j++));
                        }, 1000, peer);
                    }
                    return [2 /*return*/];
            }
        });
    });
})();
