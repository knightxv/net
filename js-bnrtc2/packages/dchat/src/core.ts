/// <reference types="node" />
import {
    Bnrtc2Client,
    Bnrtc2Controller,
    bnrtc2Controller,
} from "@bfchain/bnrtc2-client";
import { RESULT_CODE, READY_STATE } from "@bfchain/bnrtc2-client-typings";
import { sleep, unsleep, PromiseOut } from "@bfchain/util-extends-promise";
import { EasyMap } from "@bfchain/util-extends-map";
import { Bnrtc2Buffer } from "@bfchain/bnrtc2-buffer";
import {
    MessageState,
    LoginStatus,
    DCHAT_SERVICE_NAME,
    DCHAT_SERVICE_CLIENT_DPORT,
    DCHAT_SERVICE_SERVER_DPORT,
    DCHAT_SERVICE_ADDRESS_UPDATE_INTERVAL,
    ServiceMessageType,
    DCHAT_MESSAGE_TIMEOUT_DEFAULT,
} from "@bfchain/bnrtc2-dchat-typings";
import {
    isValidAddress,
    isValidDport,
    isValidMessageTimeout,
    stringSetToBuffer,
    getByteArray,
    getStringArray,
} from "./util";

function nullFunction(...rest: any[]) { }
const log = nullFunction;
const info = nullFunction;
const error = nullFunction;

enum MessageType {
    DATA = 1,
    ACK = 2,
}

type dataSender = (
    address: string,
    dport: string,
    buf: Bnrtc2Buffer,
    devid?: string,
    src?: string
) => Promise<RESULT_CODE>;

// Messgage Format
// +-------------------------------------+
// + 1byte | 4bytes | 4bytes | 0..nbytes +
// +-------------------------------------+
// + type  | msgId  |  mahic | payload   +
// +-------------------------------------+
const MESSAGE_HEADER_SIZE = 9;
export class Dchat implements DCHAT.CHAT {
    // 消息超时时间(单位: 秒)
    private _messageTimeoutSeconds: number = DCHAT_MESSAGE_TIMEOUT_DEFAULT;
    private _bnrtc2Client: Bnrtc2Client;
    private _bnrtc2Controller: Bnrtc2Controller;
    private _messageHandlersMap: Map<string, Set<DCHAT.MessageHandler>> =
        new Map();
    private _messageHandlersBoundStateMap: Map<string, boolean> = new Map();
    private _messageHanderBound = this._messageHandler.bind(this);
    private _msgIdAcc: Uint32Array = new Uint32Array(1);
    private isClientClosed: boolean = true;
    private isClientOpened: boolean = false;
    private isClosed: boolean = false;
    private stateChangeHandler?: DCHAT.StateChangeHandler;
    private magic: number = Math.floor(Math.random() * 0xffffffff);
    private loginStatusHandler?: DCHAT.LoginStateChangeHandler;
    private loginAddresses: Set<string> = new Set();
    private defaultAddress?: string;
    private serviceAddress?: string;
    private serviceAddressUpdateTime: number = 0;
    private friends: Map<string, Map<string, boolean>> = new Map();
    private onlineAddress: Set<string> = new Set();
    private checkTimerId: NodeJS.Timer;
    private _unConfirmMsgMap = EasyMap.from<
        number,
        PromiseOut<MessageState>,
        number
    >({
        creater: (msgId: number) => {
            const task = new PromiseOut<MessageState>();
            const tickTimeout = sleep(
                this._messageTimeoutSeconds * 1000,
                () => {
                    // 使用resolve，不使用reject
                    task.resolve(MessageState.Timeout);
                }
            );
            task.onSuccess(() => {
                // Resolve时取消超时定时器
                unsleep(tickTimeout);
            });
            task.onFinished(() => {
                // 完成时删除自己
                this._unConfirmMsgMap.tryDelete(msgId);
            });
            return task;
        },
    });

    constructor(private apiHost?: string, private apiPort?: number) {
        this._bnrtc2Client = this.newkBnrtc2Client();
        if (apiHost != undefined || apiPort != undefined) {
            this._bnrtc2Controller = new Bnrtc2Controller(apiHost, apiPort);
        } else {
            this._bnrtc2Controller = bnrtc2Controller;
        }

        this.checkTimerId = setInterval(() => {
            this.checkDportBinding.bind(this)();
            this.checkServiceBinding.bind(this)();
            this.updateServiceAddress.bind(this)();
        }, 60000);

        this.onMessage(
            DCHAT_SERVICE_CLIENT_DPORT,
            this.serviceMessageHandler.bind(this)
        );
    }

    private checkDportBinding() {
        if (this.isClientOpened) {
            // rebind onData
            for (const [dport, bound] of this._messageHandlersBoundStateMap) {
                if (!bound) {
                    this._bnrtc2Client
                        .onData(dport, this._messageHanderBound)
                        .then(() =>
                            this._messageHandlersBoundStateMap.set(dport, true)
                        )
                        .catch(() => {
                            return;
                        });
                }
            }
        }
    }

    private checkServiceBinding() {
        if (this.isClientOpened) {
            if (this.loginAddresses.size > 0) {
                this.sendToService(
                    ServiceMessageType.KEEP_ALIVE,
                    stringSetToBuffer(this.loginAddresses)
                ).catch(() => {
                    return;
                });
            }
            for (const [address, _] of this.friends) {
                this.doUpdateFriends(address);
            }
        }
    }

    private async resetFriendsStatus(address: string) {
        const friendMap = this.friends.get(address);
        if (friendMap === undefined) {
            return;
        }
        friendMap.forEach((_, key) => {
            friendMap.set(key, false);
        });
    }

    private async doUpdateFriends(address: string) {
        const friendMap = this.friends.get(address);
        if (friendMap === undefined) {
            return;
        }
        const toAddFriends: Set<string> = new Set();
        friendMap.forEach((added: boolean, friend: string) => {
            if (!added) {
                toAddFriends.add(friend);
            }
        });

        if (toAddFriends.size > 0) {
            this.sendToService(
                ServiceMessageType.ADD_FRIENDS,
                stringSetToBuffer(toAddFriends),
                address
            )
                .then((res) => {
                    if (res == MessageState.Success) {
                        for (const friend of toAddFriends) {
                            friendMap.set(friend, true);
                        }
                    }
                })
                .catch(() => {
                    return;
                });
        }
    }

    private async doUpdateServiceAddress() {
        await this._bnrtc2Controller
            .getServiceAddresses(DCHAT_SERVICE_NAME)
            .then((addresses) => {
                if (addresses?.length > 0) {
                    this.serviceAddress = addresses[0];
                    this.serviceAddressUpdateTime = Date.now();
                }
            });
    }

    private async updateServiceAddress() {
        const now = Date.now();
        if (
            now - this.serviceAddressUpdateTime >
            DCHAT_SERVICE_ADDRESS_UPDATE_INTERVAL
        ) {
            this.doUpdateServiceAddress();
        }
    }

    private async getServiceAddress(): Promise<string | undefined> {
        if (this.serviceAddress === undefined) {
            await this.doUpdateServiceAddress();
        }
        return this.serviceAddress;
    }

    private newkBnrtc2Client(): Bnrtc2Client {
        this.isClientClosed = false;
        this.isClientOpened = false;
        const bnrtc2Client = new Bnrtc2Client(this.apiHost, this.apiPort);
        bnrtc2Client.onClose.attach(() => {
            this.isClientClosed = true;
            this.isClientOpened = false;
            for (const [dport] of this._messageHandlersBoundStateMap) {
                this._messageHandlersBoundStateMap.set(dport, false);
            }

            if (this.stateChangeHandler) {
                this.stateChangeHandler(READY_STATE.CLOSE);
            }
        });
        bnrtc2Client.onOpen.attach(async () => {
            this.isClientOpened = true;
            this.isClientClosed = false;
            // rebind onData
            this.checkDportBinding();
            if (this.stateChangeHandler) {
                this.stateChangeHandler(READY_STATE.OPEN);
            }
        });
        return bnrtc2Client;
    }

    private checkBnrtc2Client() {
        if (this.isClientClosed) {
            this.isClientClosed = false;
            this._bnrtc2Client = this.newkBnrtc2Client();
        }
    }

    private getMessageId(): number {
        return this._msgIdAcc[0]++;
    }

    private buildAckMsg(msgId: number, magic: number): Bnrtc2Buffer {
        const buf = Bnrtc2Buffer.create(MESSAGE_HEADER_SIZE);
        buf.pushU8(MessageType.ACK);
        buf.pushU32(msgId);
        buf.pushU32(magic);

        return buf;
    }

    private resultCodeToMessageState(resultCode: RESULT_CODE): MessageState {
        switch (resultCode) {
            case RESULT_CODE.SUCCESS:
                return MessageState.Success;
            case RESULT_CODE.FAILED:
                return MessageState.Failed;
            case RESULT_CODE.OFFLINE:
                return MessageState.Offline;
            case RESULT_CODE.DPORT_CONFLICT:
                return MessageState.DportConflict;
            case RESULT_CODE.DPORT_UNBOUND:
                return MessageState.DportUnbound;
            case RESULT_CODE.CONNECT_FAILED:
                return MessageState.ConnectFailed;
            case RESULT_CODE.FORMAT_ERR:
                return MessageState.FormatError;
            case RESULT_CODE.TIMEOUT:
                return MessageState.Timeout;
            default:
                return MessageState.Unknown;
        }
    }

    private decodeServiceMessageFromBuffer(
        buf: Bnrtc2Buffer
    ): Bnrtc2.Bnrtc2Data {
        const dst = buf.pullStr();
        const src = buf.pullStr();
        const dport = buf.pullStr();
        const data = buf.data();

        return {
            address: src,
            dport,
            data,
            dst,
            devid: "",
            isSync: false,
        };
    }

    private encodeServiceMessageToBuffer(
        address: /* target */ string,
        dport: string,
        buffer: Uint8Array | Bnrtc2Buffer,
        src: string
    ): Bnrtc2Buffer {
        if (buffer instanceof Uint8Array) {
            buffer = Bnrtc2Buffer.from(buffer);
        } else if (buffer == undefined) {
            buffer = Bnrtc2Buffer.create(0);
        }
        buffer.putStr(dport);
        buffer.putStr(src);
        buffer.putStr(address);
        return buffer;
    }

    // 发送消息
    private async _send(
        address: /* target */ string,
        dport: string,
        data: Uint8Array | Bnrtc2Buffer,
        sender: dataSender,
        devid?: string,
        src?: string
    ): Promise<MessageState> {
        if ((src === undefined || src === "") && this.defaultAddress !== undefined) {
            src = this.defaultAddress;
        }

        log(
            "send: address(%s) dport(%s) data.len(%d), devid(%s), src(%s)",
            address,
            dport,
            data.length,
            devid,
            src
        );
        const checkRes = this._checkAddressDport(address, dport, true);
        if (checkRes !== MessageState.Success) {
            return checkRes;
        }

        this.checkBnrtc2Client();
        if (!this.isClientOpened) {
            error("not connected");
            return MessageState.ConnectFailed;
        }
        // try to send service if not online
        if (
            !this.onlineAddress.has(address) &&
            dport != DCHAT_SERVICE_SERVER_DPORT
        ) {
            if (src == undefined) {
                return MessageState.NoAddress;
            }

            const buf = this.encodeServiceMessageToBuffer(
                address,
                dport,
                data,
                src
            );
            const res = await this.sendToService(
                ServiceMessageType.MESSAGE,
                buf,
                src
            );
            if (res === MessageState.Success) {
                return MessageState.Success;
            }
        }

        const msgId = this.getMessageId();
        let buf = data;
        if (buf instanceof Uint8Array) {
            buf = Bnrtc2Buffer.from(buf);
        }

        buf.putU32(this.magic);
        buf.putU32(msgId);
        buf.putU8(MessageType.DATA);
        log(
            "send DATA: address(%s) dport(%s) msgid(%d) data.len(%d) data(%s...)",
            address,
            dport,
            msgId,
            data.length,
            buf.data().subarray(0, 16)
        );
        const t = this._unConfirmMsgMap.forceGet(msgId);
        const res = sender(address, dport, buf, devid, src);
        return res
            .then(() => {
                return t.promise;
            })
            .catch((err) => {
                error("error", err);
                // send failed, treat target as offline
                this.delOnlineAddresses([address]);
                t.resolve(MessageState.Failed);
                return this.resultCodeToMessageState(err);
            });

        // const p = new PromiseOut<MessageState>();
        // res.then((resultCode: ResultCode) => {
        //     log("send resultCode: %d", resultCode);
        //     if (resultCode === ResultCode.Success) {
        //         const t = this._unConfirmMsgMap.forceGet(msgId);
        //         t.onSuccess((state: MessageState)=>{
        //             p.resolve(state)
        //         })
        //     } else {
        //         p.resolve(this.resultCodeToMessageState(resultCode));
        //     }
        // });
        // return p.promise;
    }

    public async send(
        address: /* target */ string,
        dport: string,
        data: Uint8Array | Bnrtc2Buffer,
        devid?: string,
        src?: string
    ) {
        const sender = (
            address: string,
            dport: string,
            buf: Bnrtc2Buffer,
            devid?: string,
            src?: string
        ) => {
            return this._bnrtc2Client.sendAll(address, dport, buf, devid, src);
        };

        return this._send(address, dport, data, sender, devid, src); // send to all
    }

    public async sendOne(
        address: string,
        dport: string,
        data: Uint8Array | Bnrtc2Buffer,
        devid?: string,
        src?: string
    ) {
        const sender = (
            address: string,
            dport: string,
            buf: Bnrtc2Buffer,
            devid?: string,
            src?: string
        ) => {
            return this._bnrtc2Client.send(address, dport, buf, devid, src);
        };
        return this._send(address, dport, data, sender, devid, src); // send to one
    }

    public async sync(
        dport: string,
        data: Uint8Array | Bnrtc2Buffer,
        src?: string
    ) {
        const sender = (
            address: string,
            dport: string,
            buf: Bnrtc2Buffer,
            devid?: string,
            src?: string
        ) => {
            return this._bnrtc2Client.sync(dport, buf, src);
        };
        return this._send("", dport, data, sender, undefined, src); // sync
    }

    private confirmMessage(id: number, magic: number): void {
        if (magic != this.magic) {
            // not for me
            return;
        }
        const task = this._unConfirmMsgMap.tryGet(id);
        if (task) {
            task.resolve(MessageState.Success);
        }
    }

    private sendAckMessage(
        id: number,
        magic: number,
        msg: Bnrtc2.Bnrtc2Data
    ): void {
        const ackMsg = this.buildAckMsg(id, magic);
        log(
            "send ACK: devid(%s) address(%s) dport(%s) msgid(%d)",
            msg.devid,
            msg.address,
            msg.dport,
            id
        );
        let dport = msg.dport;
        // exchange dport for service
        if (msg.dport == DCHAT_SERVICE_SERVER_DPORT) {
            dport = DCHAT_SERVICE_CLIENT_DPORT;
        } else if (msg.dport == DCHAT_SERVICE_CLIENT_DPORT) {
            dport = DCHAT_SERVICE_SERVER_DPORT;
        }
        this._bnrtc2Client
            .send(msg.address, dport, ackMsg, msg.devid, msg.dst)
            .catch(() => {
                return;
            });
    }

    private processDataMessage(msg: Bnrtc2.Bnrtc2Data, data: Uint8Array): void {
        const handlerSet = this._messageHandlersMap.get(msg.dport);
        if (handlerSet === undefined) {
            log("no handlers for dport(%s)", msg.dport);
            return;
        }
        for (const handler of handlerSet) {
            const res = handler(
                msg.address,
                msg.dport,
                data,
                msg.devid,
                msg.dst,
                msg.isSync
            );
            if (!res) {
                break;
            }
        }
    }
    private addOnlineAddresses(addresses: string[]): void {
        if (addresses.length === 0) {
            return;
        }
        for (const address of addresses) {
            this.onlineAddress.add(address);
            info("online: %s", address);
        }
        if (this.loginStatusHandler != undefined) {
            this.loginStatusHandler(addresses, LoginStatus.Online);
        }
    }

    private delOnlineAddresses(addresses: string[]): void {
        if (addresses.length === 0) {
            return;
        }

        for (const address of addresses) {
            this.onlineAddress.delete(address);
            info("offline: %s", address);
        }
        if (this.loginStatusHandler != undefined) {
            this.loginStatusHandler(addresses, LoginStatus.Offline);
        }
    }

    private serviceMessageHandler(
        address: string,
        dport: string,
        data: Uint8Array,
        devid?: string,
        src?: string,
        isSync?: boolean
    ): boolean {
        const buf = Bnrtc2Buffer.from(data);
        if (buf.length < 1) {
            error("invalid buffer len (%d)", buf.length);
            return false;
        }

        const mt = buf.pullU8();
        info("recv service message: mt(%d)", mt);
        switch (mt) {
            case ServiceMessageType.ONLINE:
                const onlineAddresses = getStringArray(buf);
                this.addOnlineAddresses(onlineAddresses);
                break;
            case ServiceMessageType.OFFLINE:
                const offlineAddresses = getStringArray(buf);
                this.delOnlineAddresses(offlineAddresses);
                break;
            case ServiceMessageType.MESSAGE:
                const messages = getByteArray(buf);
                info("got messages %d", messages.length);
                if (messages.length > 0) {
                    for (const message of messages) {
                        const newBuf = Bnrtc2Buffer.from(message);
                        const msg = this.decodeServiceMessageFromBuffer(newBuf);
                        this.processDataMessage(msg, newBuf.data());
                    }
                }
                break;
            case ServiceMessageType.ONLINE_ACK:
                const onlineAckAddresses = getStringArray(buf);
                for (const address of onlineAckAddresses) {
                    this.resetFriendsStatus(address);
                    this.doUpdateFriends(address);
                }
                break;

            default: // unknown
                error("unknown service message type(%d)", mt);
        }
        return true;
    }

    private _messageHandler(msg: Bnrtc2.Bnrtc2Data): void {
        const buf = Bnrtc2Buffer.from(msg.data);
        if (buf.length < MESSAGE_HEADER_SIZE) {
            error("invalid buffer len (%d)", buf.length);
            return;
        }
        const type: MessageType = buf.pullU8();
        const msgId = buf.pullU32();
        const magic = buf.pullU32();
        log(
            "recv: src(%s) dport(%s) type(%s) msgid(%d) isSync(%d) data.len(%d) data(%s...)",
            msg.address,
            msg.dport,
            type === MessageType.DATA ? "DATA" : "ACK",
            msgId,
            msg.isSync,
            buf.length,
            buf.data().subarray(0, 16)
        );

        if (!this.onlineAddress.has(msg.address)) {
            for (const [_, frineds] of this.friends) {
                if (frineds.has(msg.address)) {
                    this.addOnlineAddresses([msg.address]);
                    break;
                }
            }
        }

        if (type === MessageType.ACK) {
            this.confirmMessage(msgId, magic);
        } else if (type === MessageType.DATA) {
            if (
                isValidAddress(msg.address) === false ||
                isValidDport(msg.dport, false) === false
            ) {
                error(
                    "invalid address(%s) or dport(%s)",
                    msg.address,
                    msg.dport
                );
                return;
            }
            if (!msg.isSync) {
                // send ack if is not synced msg
                this.sendAckMessage(msgId, magic, msg);
            }
            this.processDataMessage(msg, buf.data());
        }

        return;
    }

    public onStateChange(handler: DCHAT.StateChangeHandler) {
        const self = this;
        this.stateChangeHandler = async function (state: READY_STATE) {
            if (state == READY_STATE.OPEN) {
                self.checkServiceBinding();
            }
            handler(state);
        };
    }

    // 注册接收消息回调
    public async onMessage(
        dport: string,
        handler: DCHAT.MessageHandler
    ): Promise<boolean> {
        const chechRes = this._checkDport(dport, false);
        if (chechRes !== MessageState.Success) {
            return false;
        }

        let handlerSet = this._messageHandlersMap.get(dport);
        if (handlerSet === undefined) {
            this._messageHandlersBoundStateMap.set(dport, false);
            if (this.isClientOpened) {
                const result = await this._bnrtc2Client
                    .onData(dport, this._messageHanderBound)
                    .catch((code: RESULT_CODE) => {
                        return code;
                    });
                if (result == RESULT_CODE.SUCCESS) {
                    this._messageHandlersBoundStateMap.set(dport, true);
                }
            }
            handlerSet = new Set<DCHAT.MessageHandler>();
            this._messageHandlersMap.set(dport, handlerSet);
        }

        log("add dport(%s) handler ok", dport);
        handlerSet.add(handler);
        return true;
    }

    // 取消接收消息回调
    public async offMessage(
        dport: string,
        handler: DCHAT.MessageHandler
    ): Promise<boolean> {
        const chechRes = this._checkDport(dport, false);
        if (chechRes !== MessageState.Success) {
            return false;
        }

        const handlerSet = this._messageHandlersMap.get(dport);
        if (handlerSet === undefined || handlerSet.size === 0) {
            return true;
        }

        handlerSet.delete(handler);
        if (handlerSet.size != 0) {
            return true;
        }

        log("unregist dport(%d)", dport);
        this._messageHandlersMap.delete(dport);
        this._messageHandlersBoundStateMap.delete(dport);
        if (this.isClientOpened) {
            const result = await this._bnrtc2Client
                .offData(dport, this._messageHanderBound)
                .catch((code: RESULT_CODE) => {
                    return code;
                });
            return result === RESULT_CODE.SUCCESS;
        }

        return true;
    }

    // 获取消息超时时间(单位: 秒)
    public getMessageTimeout(): number {
        return this._messageTimeoutSeconds;
    }

    // 设置消息超时时间(单位: 秒)
    public setMessageTimeout(timeout: number): boolean {
        if (!isValidMessageTimeout(timeout)) {
            error("invalid timeout(%d)", timeout);
            return false;
        }

        log("set timeout(%d) ok", timeout);
        this._messageTimeoutSeconds = timeout;
        return true;
    }

    private async sendToService(
        mt: ServiceMessageType,
        data?: Uint8Array | string | Bnrtc2Buffer,
        src?: string
    ): Promise<MessageState> {
        log("send to service: mt(%d)", mt);
        const serviceAddress = await this.getServiceAddress();
        if (serviceAddress === undefined) {
            return MessageState.ServiceNotReady;
        }

        if (src === undefined || src.length == 0) {
            src = this.defaultAddress;
        }

        let buf: Bnrtc2Buffer;
        if (data === undefined) {
            buf = Bnrtc2Buffer.create(0);
        } else if (typeof data === "string") {
            buf = Bnrtc2Buffer.create(0);
            buf.pushStr(data);
        } else if (data instanceof Uint8Array) {
            buf = Bnrtc2Buffer.from(data);
        } else if (data instanceof Bnrtc2Buffer) {
            buf = data;
        } else {
            return MessageState.FormatError;
        }
        buf.putU8(mt);
        return this.send(
            serviceAddress,
            DCHAT_SERVICE_SERVER_DPORT,
            buf,
            undefined,
            src
        );
    }

    private _checkAddress(address: string): MessageState {
        if (this.isClosed) {
            error("dchat is closed");
            return MessageState.Closed;
        }

        if (isValidAddress(address) === false) {
            error("invalid address(%s)", address);
            return MessageState.FormatError;
        }

        return MessageState.Success;
    }

    private _checkDport(dport: string, isSent: boolean): MessageState {
        if (this.isClosed) {
            error("dchat is closed");
            return MessageState.Closed;
        }

        if (isValidDport(dport, isSent) === false) {
            error("invalid dport(%s)", dport);
            return MessageState.FormatError;
        }

        return MessageState.Success;
    }

    private _checkAddressDport(
        address: string,
        dport: string,
        isSent: boolean
    ): MessageState {
        if (this.isClosed) {
            error("dchat is closed");
            return MessageState.Closed;
        }

        if (isValidAddress(address) === false) {
            error("invalid address(%s)", address);
            return MessageState.FormatError;
        }

        if (isValidDport(dport, isSent) === false) {
            error("invalid dport(%s)", dport);
            return MessageState.FormatError;
        }

        return MessageState.Success;
    }

    // 是否在线
    public async isOnline(address: string): Promise<boolean> {
        if (this._checkAddress(address) !== MessageState.Success) {
            return Promise.resolve(false);
        }

        return this._bnrtc2Controller.isOnline(address);
    }

    // 建立通信连接
    public async connect(address: string): Promise<boolean> {
        if (this._checkAddress(address) !== MessageState.Success) {
            return Promise.resolve(false);
        }

        return this._bnrtc2Controller.connectAddress(address);
    }

    // 登录本地地址
    public async login(address: string): Promise<MessageState> {
        const checkRes = this._checkAddress(address);
        if (checkRes !== MessageState.Success) {
            return checkRes;
        }

        log("login: address(%s)", address);
        if (this.loginAddresses.has(address)) {
            return MessageState.Success;
        }
        await this._bnrtc2Controller.bindAddress(address);
        this.loginAddresses.add(address);
        this.defaultAddress = address;
        return await this.sendToService(
            ServiceMessageType.ONLINE,
            undefined,
            address
        );
    }

    // 登出本地地址
    public async logout(address: string): Promise<MessageState> {
        const checkRes = this._checkAddress(address);
        if (checkRes !== MessageState.Success) {
            return checkRes;
        }

        log("logout: address(%s)", address);
        const ok = this.loginAddresses.has(address);
        if (!ok) {
            return MessageState.Success;
        }
        this.loginAddresses.delete(address);
        if (this.defaultAddress === address) {
            this.defaultAddress = undefined;
        }
        for (const address of this.loginAddresses.values()) {
            // use first address as default
            this.defaultAddress = address;
            break;
        }
        const res = await this.sendToService(
            ServiceMessageType.OFFLINE,
            undefined,
            address
        );
        // @note: must send to service before unbind
        await this._bnrtc2Controller.unbindAddress(address);
        return res;
    }

    // 添加关注地址
    public async addFriends(
        address: string,
        friends: Array<string>
    ): Promise<MessageState> {
        const checkRes = this._checkAddress(address);
        if (checkRes !== MessageState.Success) {
            return checkRes;
        }

        let friendMap = this.friends.get(address);
        if (friendMap === undefined) {
            friendMap = new Map();
            this.friends.set(address, friendMap);
        }

        const toAddFriends: Set<string> = new Set<string>();
        const onlineFriends: string[] = [];

        for (const friend of friends) {
            if (friend === "") {
                continue;
            } else if (friendMap.has(friend)) {
                if (this.onlineAddress.has(friend)) {
                    onlineFriends.push(friend);
                }
            } else {
                toAddFriends.add(friend);
                friendMap.set(friend, false);
            }
        }

        if (onlineFriends.length != 0) {
            if (this.loginStatusHandler != undefined) {
                this.loginStatusHandler(onlineFriends, LoginStatus.Online);
            }
        }

        if (toAddFriends.size == 0) {
            return MessageState.Success;
        }
        const result = await this.sendToService(
            ServiceMessageType.ADD_FRIENDS,
            stringSetToBuffer(toAddFriends),
            address
        );
        if (result === MessageState.Success) {
            for (const friend of toAddFriends) {
                friendMap.set(friend, true);
            }
        }

        return result;
    }

    // 取消关注地址
    public async delFriends(
        address: string,
        friends: Array<string>
    ): Promise<MessageState> {
        const checkRes = this._checkAddress(address);
        if (checkRes !== MessageState.Success) {
            return checkRes;
        }

        const friendMap = this.friends.get(address);
        if (friendMap === undefined) {
            return MessageState.Success;
        }

        const toDelFriends: Set<string> = new Set<string>();
        friends.forEach((friend) => {
            if (friend != "" && friendMap.has(friend)) {
                toDelFriends.add(friend);
            }
        });

        if (toDelFriends.size == 0) {
            return MessageState.Success;
        }

        const result = await this.sendToService(
            ServiceMessageType.DEL_FRIENDS,
            stringSetToBuffer(toDelFriends),
            address
        );
        if (result === MessageState.Success) {
            for (const friend of toDelFriends) {
                friendMap.delete(friend);
            }
            if (friendMap.size == 0) {
                this.friends.delete(address);
            }
        }

        return result;
    }

    // 注册其他地址登录状态变更回调
    public onLoginStatusChange(handler: DCHAT.LoginStateChangeHandler): void {
        this.loginStatusHandler = handler;
    }

    // 关闭通道
    public close() {
        if (this.isClosed) {
            return;
        }
        this.isClosed = true;
        clearInterval(this.checkTimerId);
        this._unConfirmMsgMap.forEach((task) => {
            task.resolve(MessageState.Closed);
        });
        this._unConfirmMsgMap.clear();
        this._messageHandlersMap.clear();
        this._messageHandlersBoundStateMap.clear();
        this._bnrtc2Client.close();
    }
}
