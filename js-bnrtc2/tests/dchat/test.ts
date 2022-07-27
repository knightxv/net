import { Dchat } from "@bfchain/bnrtc2-dchat-node";
import { READY_STATE } from "@bfchain/bnrtc2-client-typings";
import {
    DCHAT_DPORT_PREFIX,
    MessageState,
    LoginStatus,
} from "@bfchain/bnrtc2-dchat-typings";
import { Bnrtc2Buffer } from "@bfchain/bnrtc2-buffer";
import * as http from "http";
import { Bnrtc2Controller } from "@bfchain/bnrtc2-client";

const DCHAT_DPORT = DCHAT_DPORT_PREFIX + "bfchain";
type DchatConfig = {
    name: string;
    localAddress: string;
    remoteAddress: string[];
    httpPort: number;
    bnrtcPort: number;
};
const configs: DchatConfig[] = [
    {
        name: "node1",
        localAddress: "C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D",
        remoteAddress: [
            "3jui8ko762DHguaNcfoHgJkWvCWsUo4ok",
            "QFbkas6RKsCeGTUQRUESGmWvEAZN8yNCE",
        ],
        httpPort: 11111,
        bnrtcPort: 19888,
    },
    {
        name: "node2",
        localAddress: "3jui8ko762DHguaNcfoHgJkWvCWsUo4ok",
        remoteAddress: [
            "C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D",
            "QFbkas6RKsCeGTUQRUESGmWvEAZN8yNCE",
        ],
        httpPort: 11112,
        bnrtcPort: 19999,
    },
    {
        name: "node2-1",
        localAddress: "3jui8ko762DHguaNcfoHgJkWvCWsUo4ok",
        remoteAddress: ["C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D", "QFbkas6RKsCeGTUQRUESGmWvEAZN8yNCE"],
        httpPort: 11113,
        bnrtcPort: 20000,
    }
];

const loginConfig: DchatConfig = {
    name: "node3",
    localAddress: "QFbkas6RKsCeGTUQRUESGmWvEAZN8yNCE",
    remoteAddress: ["3jui8ko762DHguaNcfoHgJkWvCWsUo4ok", "C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D"],
    httpPort: 11114,
    bnrtcPort: 20001,
};

function logIt(...args: any[]) {
    console.log(args);
}

async function startDchatService(
    config: DchatConfig
): Promise<{ dchat: Dchat; server: http.Server }> {
    const dchat = new Dchat("127.0.0.1", config.bnrtcPort);
    dchat.onMessage(
        DCHAT_DPORT,
        (
            address: string,
            dport: string,
            data: Uint8Array,
            devid?: string,
            src?: string,
            isSync?: boolean
        ) => {
            const msg = Bnrtc2Buffer.from(data).pullStr();
            if (isSync) {
                logIt(
                    config.name,
                    config.localAddress,
                    " got Sync message: ",
                    address,
                    dport,
                    devid,
                    src,
                    msg
                );
            } else {
                logIt(
                    config.name,
                    config.localAddress,
                    " got Send message: ",
                    address,
                    dport,
                    devid,
                    src,
                    msg
                );
            }
            return true;
        }
    );
    dchat.onLoginStatusChange(function (
        addresses: string[],
        state: LoginStatus
    ) {
        logIt(
            config.name,
            config.localAddress,
            "receive login status: ",
            state,
            addresses
        );
    });

    dchat.onStateChange(async function (state: READY_STATE) {
        logIt(
            config.name,
            config.localAddress,
            "receive ready status: ",
            state
        );
        if (state == READY_STATE.OPEN) {
            await dchat.login(config.localAddress);
            await dchat.addFriends(config.localAddress, config.remoteAddress);
        }
    });
    console.log(
        "start server",
        config.localAddress,
        config.remoteAddress,
        config.httpPort,
        config.bnrtcPort
    );
    const server = http.createServer(
        async (req: http.IncomingMessage, res: http.ServerResponse) => {
            //设置允许跨域的域名，*代表允许任意域名跨域
            res.setHeader("Access-Control-Allow-Origin", "*");
            //跨域允许的header类型
            res.setHeader(
                "Access-Control-Allow-Headers",
                "Content-type,Content-Length,Authorization,Accept,X-Requested-Width"
            );
            //跨域允许的请求方式
            res.setHeader(
                "Access-Control-Allow-Methods",
                "PUT,POST,GET,DELETE,OPTIONS"
            );
            //设置响应头信息
            res.setHeader("X-Powered-By", " 3.2.1");
            //让options请求快速返回
            if (req.method == "GET") {
                const url = new URL(req.url || "", "http://127.0.0.1");
                if (url.pathname === "/connect") {
                    const address = url.searchParams.get("address");
                    const res = await dchat.connect(address!);
                    logIt(
                        "connect remote address: ",
                        config.remoteAddress,
                        res
                    );
                } else if (url.pathname === "/message") {
                    const message = url.searchParams.get("message") || "";
                    logIt("send message : ", message);
                    const buf = Bnrtc2Buffer.create(100);
                    buf.pushStr(message);
                    for (const address of config.remoteAddress) {
                        await dchat
                            .sendOne(address, DCHAT_DPORT, buf.data())
                            .then((r: any) => {
                                logIt("send One data Code: ", r);
                                if (r == MessageState.Success) {
                                    logIt(
                                        "send One data: ",
                                        message,
                                        address,
                                        "ok"
                                    );
                                } else {
                                    logIt(
                                        "send One data ",
                                        message,
                                        config.remoteAddress,
                                        "failed",
                                        r
                                    );
                                }
                            });
                    }
                }
                return res.end();
            }
        }
    );
    server.listen(config.httpPort);
    return {
        dchat,
        server,
    };
}

function startControlService() {
    let loginDchat: Dchat | undefined = undefined;
    let loginServer: http.Server | undefined = undefined;
    const controller = new Bnrtc2Controller("127.0.0.1", loginConfig.bnrtcPort);
    http.createServer(
        async (req: http.IncomingMessage, res: http.ServerResponse) => {
            //设置允许跨域的域名，*代表允许任意域名跨域
            res.setHeader("Access-Control-Allow-Origin", "*");
            //跨域允许的header类型
            res.setHeader(
                "Access-Control-Allow-Headers",
                "Content-type,Content-Length,Authorization,Accept,X-Requested-Width"
            );
            //跨域允许的请求方式
            res.setHeader(
                "Access-Control-Allow-Methods",
                "PUT,POST,GET,DELETE,OPTIONS"
            );
            //设置响应头信息
            res.setHeader("X-Powered-By", " 3.2.1");
            //让options请求快速返回
            if (req.method == "GET") {
                const url = new URL(req.url || "", "http://127.0.0.1");
                if (url.pathname === "/login") {
                    logIt("login : ");
                    let { dchat, server } = await startDchatService(
                        loginConfig
                    );
                    loginDchat = dchat;
                    loginServer = server;
                    res.end("login success");
                } else if (url.pathname === "/logout") {
                    logIt("logout: ");
                    await loginDchat?.logout(loginConfig.localAddress);
                    await loginDchat?.close();
                    await loginServer?.close();
                    res.end("logout success");
                } else if (url.pathname === "/isonline") {
                    const address = url.searchParams.get("address");
                    const online = await controller.isOnline(address!);
                    logIt("isOnline: ", address, online);
                    res.end(online);
                } else if (url.pathname === "/info") {
                    const service = url.searchParams.get("service");
                    const info = await controller.getServiceInfo(service!);
                    logIt("serviceInfo: ", service, info);
                    res.end(JSON.stringify(info));
                } else {
                    res.end("error");
                }
            }
        }
    ).listen(10000);
    logIt("start control service: ", 10000);
}

startControlService();
for (const config of configs) {
    startDchatService(config);
}
