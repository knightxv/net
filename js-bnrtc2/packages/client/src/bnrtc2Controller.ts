import {
    Bnrtc2Api,
    DEFAULT_BASE_API_HOSTNAME,
    DEFAULT_BASE_API_PORT,
    ServiceInfo,
} from "@bfchain/bnrtc2-client-api";
export class Bnrtc2Controller implements Bnrtc2.Bnrtc2ControllerInterface {
    private readonly _api: Bnrtc2Api;
    constructor(
        apiHost: string = DEFAULT_BASE_API_HOSTNAME,
        apiPort: number = DEFAULT_BASE_API_PORT
    ) {
        this._api = new Bnrtc2Api(apiHost, apiPort);
    }
    registService(serviceInfo: ServiceInfo): Promise<boolean> {
        return this._api.registService(serviceInfo);
    }

    /**绑定本机地址 */
    async bindAddress(address: string): Promise<boolean> {
        return this._api.bindAddress(address);
    }

    /**解绑本机地址 */
    async unbindAddress(address: string): Promise<boolean> {
        return this._api.unbindAddress(address);
    }

    /**目标地址是否在线 */
    async isOnline(address: string): Promise<boolean> {
        return this._api.isOnline(address);
    }

    async addPeers(hosts: string[]): Promise<boolean> {
        return this._api.addPeers(hosts);
    }
    async deletePeers(hosts: string[]): Promise<boolean> {
        return this._api.delPeers(hosts);
    }
    async getPeers(): Promise<string[]> {
        return this._api.getPeers();
    }
    async getServiceAddresses(serviceName: string): Promise<string[]> {
        return this._api.getServiceAddresses(serviceName);
    }
    async connectAddress(address: string): Promise<boolean> {
        return this._api.connectAddress(address);
    }

    async getServerConfig(): Promise<Bnrtc2.Bnrtc2ServerConfig | undefined> {
        return this._api.getServerConfig();
    }

    async getServiceInfo(
        name: string
    ): Promise<Bnrtc2.Bnrtc2ServiceInfo | undefined> {
        return this._api.getServiceInfo(name);
    }

    async getAddressInfo(address: string): Promise<Bnrtc2.AddressInfo | undefined> {
        return this._api.getAddressInfo(address);
    }

}

export const bnrtc2Controller = new Bnrtc2Controller();
