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
/// <reference lib="dom"/>
var bnrtc2_dchat_node_1 = require("@bfchain/bnrtc2-dchat-node");
var bnrtc2_dchat_typings_1 = require("@bfchain/bnrtc2-dchat-typings");
var bnrtc2_buffer_1 = require("@bfchain/bnrtc2-buffer");
var DCHAT_DPORT = bnrtc2_dchat_typings_1.DCHAT_DPORT_PREFIX + "bfchain";
var addresses = [
    "KPYCJZKbUBtvpv4pr6UHbzLo2X7LUtRHU",
    "MQ9Dj5xcuK2hamTjucivXmgSyuJHF31ew",
    "5n1BR4q2S2qNyRbXFcEvTiqLQN37xrp2U",
    "4QiEkLrekRLupzhPK71A1vZtvfvCwid5t",
    "BydYy3JYcUVnhMuMy1d88qcSRRLE6M5fk",
    "5N2xSkensNnphrM72dEExCengAwVZJpbX",
    "Mhg7XhH6Xafb4VnUKSqGoyok4HqMxNcgy",
    "5VM1wwQauoNwtzLCDTraCrpz7Lcc21S7q",
    "BiC4ELURiyfvXd8Vi3if4MWKhTEubLJnk",
    "8zpzvHYrKPc3Sz6aJhhjStCuiEVQbXqsb",
    "GfmSKKEJ1LYVUx6T2kRf13Q8kUSrWPBCh",
    "JKE8y3nbSHyUNgVaQaKbLHBLW1edm15QA",
    "AAtAdULG2HFa18dFBUxhXhW2YsNZWdv9N",
    "7hVkQkatGhmCSGUmNiTGiYHemMGziXpxg",
    "EnR11GRrtGpzfhHjGfaXpSiQbF9Rxkjuu",
    "2jbGgTnMaXGzVBZ4GGdxQvAyy7TyjbvET",
    "5QrLn4Gqf2QfAnUic85a4rTn3EmSQKs4s",
    "C4reZkHG945uSqHJvnQjNc9yBTFQcnXtH",
    "EhUUGrUCxc9pVuAe3F1eJbGvSk2f3iLzE",
    "HDXyLKFNSdgL9uM3KwQmdjWQsbFN6jZzo",
    "3jui8ko762DHguaNcfoHgJkWvCWsUo4ok",
    "3qDbNKccx3yfVdKWP9MKbGeAv5QRXRRdh",
    "NxsVmqYD1MkoTt25gyHeLvrTmjsYkYfgs",
    "5HtdLPq9wkEqf3DJRXov3FwAgzb5TWyMF",
];
var sendSuccessCount = 0;
var sendCount = 0;
var recvCount = 0;
function sendMsg(dchat, address) {
    var pererAddress = addresses[Math.floor(Math.random() * addresses.length)];
    if (pererAddress === address) {
        return;
    }
    var msg = "I am " + sendCount;
    var buf = bnrtc2_buffer_1.Bnrtc2Buffer.create(0);
    buf.putStr(msg);
    dchat.send(pererAddress, DCHAT_DPORT, buf.data()).then(function (resCode) {
        if (resCode == bnrtc2_dchat_typings_1.MessageState.Success) {
            sendSuccessCount++;
            console.log("%s send msg '%s' to  %s, total Sendcount %d(%d) RecvCount %d", address, msg, pererAddress, sendCount, sendSuccessCount, recvCount);
        }
        else {
            console.log("%s send msg '%s' to  %s err %s ", address, msg, pererAddress, resCode);
        }
    });
    sendCount++;
}
var startPort = 20000;
var _loop_1 = function (i) {
    var localAddress = addresses[i];
    var friends = addresses.slice(0, i).concat(addresses.slice(i + 1));
    var dchat = new bnrtc2_dchat_node_1.Dchat("127.0.0.1", startPort + i);
    dchat.onMessage(DCHAT_DPORT, function (address, dport, data, devid, src, isSync) {
        var msg = bnrtc2_buffer_1.Bnrtc2Buffer.from(data).pullStr();
        recvCount++;
        console.log("Address %s got Peer %s message: %s, total Sendcount %d(%d) RecvCount %d", localAddress, address, msg, sendCount, sendSuccessCount, recvCount);
        return true;
    });
    dchat.onStateChange(function (state) {
        return __awaiter(this, void 0, void 0, function () {
            return __generator(this, function (_a) {
                switch (_a.label) {
                    case 0:
                        if (!(state == 1 /* OPEN */)) return [3 /*break*/, 3];
                        return [4 /*yield*/, dchat.login(localAddress)];
                    case 1:
                        _a.sent();
                        return [4 /*yield*/, dchat.addFriends(localAddress, friends)];
                    case 2:
                        _a.sent();
                        _a.label = 3;
                    case 3: return [2 /*return*/];
                }
            });
        });
    });
    setTimeout(function () {
        sendMsg(dchat, localAddress);
    }, Math.ceil(30000 * Math.random()));
    setInterval(function () {
        sendMsg(dchat, localAddress);
    }, Math.ceil(30000 * Math.random()) + 30000);
};
for (var i = 0; i < addresses.length; i++) {
    _loop_1(i);
}
