import { ComprotoFactory } from "@bfchain/comproto";
import { expose, wrap, Endpoint } from "./comlink/comlink";
import { Bnrtc2Client } from "@bfchain/bnrtc2-client";
import type { IEventListener } from "./comlink/protocol";
import { RESULT_CODE } from "@bfchain/bnrtc2-client-typings";
import { MessageState } from "./typing";

export * from "./typing";

function nullFunction(...rest: any[]) {}
const log = nullFunction;
const info = nullFunction;
const error = nullFunction;

// DPORT 最大长度
export const DCHAT_DPORT_LEN_MAX = 64;
// ADDRESS 最大长度 @FIXME 暂时对address只约束最大长度
export const ADDRESS_LEN_MAX = 64;
// service address 缓存时间
export const SERVICE_ADDRESSES_CACHE_TIME = 60;

const comproto = ComprotoFactory.getSingleton();

export interface IBnrtc2Client
    extends Pick<Bnrtc2Client, "send" | "onData" | "offData"> {
    controller: Pick<
        Bnrtc2Client["controller"],
        "getServiceAddresses" | "registService" | "bindAddress"
    >;
}

/// server

export class BnrtcClient {
    constructor(public bnrtc2Client: IBnrtc2Client = new Bnrtc2Client()) {}
    protected isClosed = false;
    public userBoundAddress?: string = undefined;
    get bnrtc2Controller() {
        return this.bnrtc2Client.controller;
    }
    private _checkAddress(address: string): MessageState {
        if (this.isClosed) {
            error("dchat is closed");
            return MessageState.Closed;
        }

        if (this.isValidAddress(address) === false) {
            error("invalid address(%s)", address);
            return MessageState.FormatError;
        }

        return MessageState.Success;
    }
    isValidAddress(address: string): boolean {
        if (address.length > ADDRESS_LEN_MAX) {
            return false;
        }
        return true;
    }
    public async bindAddress(address: string) {
        const checkRes = this._checkAddress(address);
        if (checkRes !== MessageState.Success) {
            return checkRes;
        }
        await this.bnrtc2Controller.bindAddress(address);
        this.userBoundAddress = address;
        return MessageState.Success;
    }
    async registerRemoteApiService<T>(name: string, service: T) {
        if (this.userBoundAddress == undefined) {
            return MessageState.NoAddress;
        }
        const serviceInfo = {
            name,
            addresses: [this.userBoundAddress],
            serviceDport: `${name}-service-server-dport`,
            clientDport: `${name}-service-client-dport`,
        };
        const registSuccess = await this.bnrtc2Client.controller.registService(
            serviceInfo
        );
        if (!registSuccess) {
            return MessageState.ServiceRegistFail;
        }
        expose(
            service,
            new BnrtcServerEndpoint(
                name,
                this.userBoundAddress,
                this.bnrtc2Client
            )
        );
        return MessageState.Success;
    }
    getRemoteApiService<T>(name: string) {
        if (this.userBoundAddress == undefined) {
            return;
        }
        const bnrtcEndpoint = new BnrtcClientEndpoint(
            name,
            this.userBoundAddress,
            this.bnrtc2Client
        );
        return wrap<T>(bnrtcEndpoint);
    }
}

interface IServiceInfo {
    name: string;
    clientDport: string;
    serviceDport: string;
}

class BnrtcServerEndpoint implements Endpoint {
    public serviceInfo: IServiceInfo;
    private handlerMap: WeakMap<IEventListener, Bnrtc2.Bnrtc2DataHandler>;
    private acceptMessageDport: string;
    private postMessageDport: string;
    constructor(
        public name: string,
        public userBoundAddress: string,
        public bnrtc2Client: IBnrtc2Client = new Bnrtc2Client()
    ) {
        this.serviceInfo = {
            name,
            serviceDport: `${name}-service-server-dport`,
            clientDport: `${name}-service-client-dport`,
        };
        this.acceptMessageDport = this.serviceInfo.serviceDport;
        this.postMessageDport = this.serviceInfo.clientDport;
        this.handlerMap = new WeakMap();
    }
    addEventListener(type: string, listener: IEventListener): void {
        const messageHandler = (evt: Bnrtc2.Bnrtc2Data) => {
            listener({
                ...evt,
                data: comproto.deserialize(evt.data),
            });
        };
        this.handlerMap.set(listener, messageHandler);
        this.bnrtc2Client.onData(this.acceptMessageDport, messageHandler);
    }
    removeEventListener(type: string, listener: IEventListener): void {
        const messageHandler = this.handlerMap.get(listener);
        if (messageHandler) {
            this.bnrtc2Client.offData(this.acceptMessageDport, messageHandler);
            this.handlerMap.delete(listener);
        }
    }
    async postMessage(message: unknown, evt: Bnrtc2.Bnrtc2Data) {
        const resultCode = await this.bnrtc2Client.send(
            evt.address,
            this.postMessageDport,
            comproto.serialize(message),
            evt.devid,
            this.userBoundAddress
        );
        if (resultCode !== RESULT_CODE.SUCCESS) {
            error("BnrtcServerEndpoint sendMessage fail");
        }
    }
}

class BnrtcClientEndpoint implements Endpoint {
    public serviceInfo: IServiceInfo;
    private handlerMap: WeakMap<IEventListener, Bnrtc2.Bnrtc2DataHandler>;
    private acceptMessageDport: string;
    private postMessageDport: string;
    constructor(
        public name: string,
        public userBoundAddress: string,
        public bnrtc2Client: IBnrtc2Client = new Bnrtc2Client()
    ) {
        this.serviceInfo = {
            name,
            serviceDport: `${name}-service-server-dport`,
            clientDport: `${name}-service-client-dport`,
        };
        this.acceptMessageDport = this.serviceInfo.clientDport;
        this.postMessageDport = this.serviceInfo.serviceDport;
        this.handlerMap = new WeakMap();
    }
    addEventListener(type: string, listener: IEventListener): void {
        const messageHandler = (evt: Bnrtc2.Bnrtc2Data) => {
            listener({
                ...evt,
                data: comproto.deserialize(evt.data),
            });
        };
        this.handlerMap.set(listener, messageHandler);
        this.bnrtc2Client.onData(this.acceptMessageDport, messageHandler);
    }
    removeEventListener(type: string, listener: IEventListener): void {
        const messageHandler = this.handlerMap.get(listener);
        if (messageHandler) {
            this.bnrtc2Client.offData(this.acceptMessageDport, messageHandler);
        }
    }
    serviceAddressesCacheMap: Map<string, string[]> = new Map();
    async getServiceAddresses(serviceName: string) {
        const cacheAddresses = this.serviceAddressesCacheMap.get(serviceName);
        if (cacheAddresses) {
            return cacheAddresses;
        }
        const addresses =
            await this.bnrtc2Client.controller.getServiceAddresses(serviceName);
        this.serviceAddressesCacheMap.set(serviceName, addresses);
        /// cache 60s
        setTimeout(() => {
            this.serviceAddressesCacheMap.delete(serviceName);
        }, SERVICE_ADDRESSES_CACHE_TIME * 1000);
        return addresses;
    }
    async postMessage(message: unknown) {
        const addresses = await this.getServiceAddresses(this.name);
        for (const address of addresses) {
            const resultCode = await this.bnrtc2Client.send(
                address,
                this.postMessageDport,
                comproto.serialize(message),
                undefined,
                this.userBoundAddress
            );
            if (resultCode === RESULT_CODE.SUCCESS) {
                return;
            }
        }
        throw new ReferenceError(
            `client postMessage fail, serverName:${
                this.name
            }, addresses:${addresses.join(",")}`
        );
    }
}
