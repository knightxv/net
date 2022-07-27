import "@bfchain/bnrtc2-client-typings";

import {
    DEFAULT_BASE_API_HOSTNAME,
    DEFAULT_BASE_API_PORT,
    API_PATH,
} from "./const";
import { bnrtc2Global } from "./global#";

export interface ChannelConnectedEvent {
    channelId: number;
    localNatName: string;
    remoteNatName: string;
    bnwsUrl: string;
    bnrtcSource: string;
}

export interface ChannelDisconnectedEvent {
    channelId: number;
    localNatName: string;
    remoteNatName: string;
}

export interface ServiceInfo {
    name: string;
    addresses: string[];
    serviceDport: string;
    clientDport: string;
    signature: string;
}

export class Bnrtc2Api {
    constructor(
        private readonly hostname: string = DEFAULT_BASE_API_HOSTNAME,
        private readonly port: number = DEFAULT_BASE_API_PORT
    ) { }

    get baseWsUrl() {
        return `ws://${this.hostname}:${this.port}`;
    }

    get baseHttpUrl() {
        return `http://${this.hostname}:${this.port}`;
    }

    httpFetch(url: string) {
        const reqInit: RequestInit = {
            method: "GET",
            headers: {
                "Content-Type": "text/plain",
            },
        };
        return bnrtc2Global.fetch(url, reqInit);
    }

    private _getUrl(baseUrl: string, path: string, query?: any) {
        const searchParam = new URLSearchParams();

        if (query) {
            Object.getOwnPropertyNames(query).forEach((key: string) => {
                searchParam.append(key, query[key] as string);
            });
        }

        const queryStr = searchParam.toString();
        if (queryStr.length !== 0) {
            return `${baseUrl}${path}?${queryStr}`;
        } else {
            return `${baseUrl}${path}`;
        }
    }

    private async httpBoolResult(res: Response): Promise<boolean> {
        try {
            if (!res.ok) {
                console.log("Http Error:", await res.text());
                return false;
            }
            const result = await res.json();
            return result?.result == true;
        } catch (e) {
            console.log("Http Error:", e);
            return false;
        }
    }

    private async httpJson(res: Response): Promise<any> {
        try {
            if (!res.ok) {
                console.log("Http Error:", await res.text());
                return undefined;
            }
            return await res.json();
        } catch (e) {
            console.log("Http Error:", e);
            return undefined;
        }
    }

    private httpFetchError<T>(e: Error, res: T): T {
        console.log("Http Error:", e);
        return res;
    }

    connectChannel() {
        const url = this._getUrl(
            this.baseWsUrl,
            API_PATH.WS_API_PATH_SERVER_CONNECT
        );
        return new bnrtc2Global.Channel(url);
    }

    disconnectChannel(channel: Bnrtc2.Channel, code?: number, reason?: string) {
        channel.close(code, reason);
    }

    async bindAddress(address: string) {
        const url = this._getUrl(
            this.baseHttpUrl,
            API_PATH.HTTP_API_PATH_SERVER_BIND_ADDRESS,
            { address }
        );
        return this.httpFetch(url)
            .then(async (res) => {
                return await this.httpBoolResult(res);
            })
            .catch((e: Error) => {
                return this.httpFetchError(e, false);
            });
    }

    async unbindAddress(address: string) {
        const url = this._getUrl(
            this.baseHttpUrl,
            API_PATH.HTTP_API_PATH_SERVER_UNBIND_ADDRESS,
            { address }
        );
        return this.httpFetch(url)
            .then(async (res) => {
                return await this.httpBoolResult(res);
            })
            .catch((e: Error) => {
                return this.httpFetchError(e, false);
            });
    }

    async isOnline(address: string) {
        const url = this._getUrl(
            this.baseHttpUrl,
            API_PATH.HTTP_API_PATH_SERVER_IS_ONLINE,
            { address }
        );
        return this.httpFetch(url)
            .then(async (res) => {
                return await this.httpBoolResult(res);
            })
            .catch((e: Error) => {
                return this.httpFetchError(e, false);
            });
    }

    async getServerConfig(): Promise<Bnrtc2.Bnrtc2ServerConfig | undefined> {
        const url = this._getUrl(
            this.baseHttpUrl,
            API_PATH.HTTP_API_PATH_SERVER_GET_SERVER_CONFIG
        );
        return this.httpFetch(url)
            .then(async (res) => {
                return await this.httpJson(res);
            })
            .catch((e: Error) => {
                return this.httpFetchError(e, undefined);
            });
    }

    async getServiceInfo(
        service: string
    ): Promise<Bnrtc2.Bnrtc2ServiceInfo | undefined> {
        const url = this._getUrl(
            this.baseHttpUrl,
            API_PATH.HTTP_API_PATH_SERVER_GET_SERVICE_INFO,
            { service }
        );
        return this.httpFetch(url)
            .then(async (res) => {
                return await this.httpJson(res);
            })
            .catch((e: Error) => {
                return this.httpFetchError(e, undefined);
            });
    }

    async getServiceAddresses(service: string): Promise<string[]> {
        const url = this._getUrl(
            this.baseHttpUrl,
            API_PATH.HTTP_API_PATH_SERVER_GET_SERVICE_ADDRESSES,
            { service }
        );
        return this.httpFetch(url)
            .then(async (res) => {
                return (await this.httpJson(res)) || [];
            })
            .catch((e: Error) => {
                return this.httpFetchError(e, []);
            });
    }

    async getAddressInfo(address: string): Promise<Bnrtc2.AddressInfo | undefined> {
        const url = this._getUrl(
            this.baseHttpUrl,
            API_PATH.HTTP_API_PATH_SERVER_GET_ADDRESSINFO,
            { address }
        );
        return this.httpFetch(url)
            .then(async (res) => {
                return (await this.httpJson(res));
            })
            .catch((e: Error) => {
                return this.httpFetchError(e, undefined);
            });
    }

    async registService(serviceInfo: ServiceInfo): Promise<boolean> {
        const url = this._getUrl(
            this.baseHttpUrl,
            API_PATH.HTTP_API_PATH_SERVER_REGIST_SERVICE,
            {
                name: serviceInfo.name,
                service_dport: serviceInfo.serviceDport,
                client_dport: serviceInfo.clientDport,
                addresses: serviceInfo.addresses.join(","),
                signature: serviceInfo.signature,
            }
        );
        return this.httpFetch(url)
            .then(async (res) => {
                return (await this.httpJson(res)) || [];
            })
            .catch((e: Error) => {
                return this.httpFetchError(e, []);
            });
    }

    async connectAddress(address: string): Promise<boolean> {
        const url = this._getUrl(
            this.baseHttpUrl,
            API_PATH.HTTP_API_PATH_SERVER_CONNECT_ADDRESS,
            { address }
        );
        return this.httpFetch(url)
            .then(async (res) => {
                return await this.httpBoolResult(res);
            })
            .catch((e: Error) => {
                return this.httpFetchError(e, false);
            });
    }

    async addPeers(hosts: string[]) {
        const url = this._getUrl(
            this.baseHttpUrl,
            API_PATH.HTTP_API_PATH_SERVER_ADD_PEERS,
            { hosts }
        );
        return this.httpFetch(url)
            .then(async (res) => {
                return await this.httpBoolResult(res);
            })
            .catch((e: Error) => {
                return this.httpFetchError(e, false);
            });
    }
    async delPeers(hosts: string[]) {
        const url = this._getUrl(
            this.baseHttpUrl,
            API_PATH.HTTP_API_PATH_SERVER_DEL_PEERS,
            { hosts }
        );
        return this.httpFetch(url)
            .then(async (res) => {
                return await this.httpBoolResult(res);
            })
            .catch((e: Error) => {
                return this.httpFetchError(e, false);
            });
    }
    async getPeers(): Promise<string[]> {
        const url = this._getUrl(
            this.baseHttpUrl,
            API_PATH.HTTP_API_PATH_SERVER_GET_PEERS
        );
        return this.httpFetch(url)
            .then(async (res) => {
                return (await this.httpJson(res)) || [];
            })
            .catch((e: Error) => {
                return this.httpFetchError(e, []);
            });
    }
}
