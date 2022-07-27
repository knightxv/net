/// <reference lib="dom"/>

declare namespace Bnrtc2 {
    interface Bnrtc2ServerConfig {
        httpPort: number;
    }

    interface Bnrtc2ServiceInfo {
        [key: string]: any;
    }

    interface EndPoint {
        name: string;
        id: string;
    }
    interface AddressInfo {
        address: string;
        endpoints: EndPoint[];
    }

    type RESULT_CODE = import("./const").RESULT_CODE;
    interface Bnrtc2ControllerInterface {
        /**绑定本机地址 */
        bindAddress(address: string): Promise<boolean>;
        /**目标地址是否在线 */
        isOnline(address: string): Promise<boolean>;
        /**添加节点 */
        addPeers(hosts: string[]): Promise<boolean>;
        /**删除节点 */
        deletePeers(hosts: string[]): Promise<boolean>;
        /**获取节点 */
        getPeers(): Promise<string[]>;
        /**获取服务地址 */
        getServiceAddresses(serviceName: string): Promise<string[]>;
        registService(serviceInfo: Bnrtc2ServiceInfo): Promise<boolean>;
        /**建立地址连接 */
        connectAddress(address: string): Promise<boolean>;
        /**获取Server配置 */
        getServerConfig(): Promise<Bnrtc2.Bnrtc2ServerConfig | undefined>;
        /**获取服务信息 */
        getServiceInfo(
            name: string
        ): Promise<Bnrtc2.Bnrtc2ServiceInfo | undefined>;
        /**获取地址信息 */
        getAddressInfo(address: string): Promise<Bnrtc2.AddressInfo | undefined>;
    }

    interface Bnrtc2Client {
        readonly onOpen: import("@bfcs/util-evt").StatefulAttacher<boolean>;
        readonly onClose: import("@bfcs/util-evt").StatefulAttacher<boolean>;
        readonly onError: import("@bfcs/util-evt").Attacher<Error>;
        readonly controller: Bnrtc2ControllerInterface;
        close(code?: number, reason?: string): void;
        send(
            address: string,
            dport: string,
            data: Bnrtc2.Bnrtc2Buffer | Uint8Array,
            devid?: string,
            src?: string
        ): Promise<RESULT_CODE>;
        broadcast(
            dport: string,
            data: Bnrtc2.Bnrtc2Buffer | Uint8Array,
            src?: string
        ): Promise<RESULT_CODE>;
        sync(
            dport: string,
            data: Bnrtc2.Bnrtc2Buffer | Uint8Array,
            src?: string
        ): Promise<RESULT_CODE>;
        onData(dport: string, handler: Bnrtc2DataHandler): Promise<RESULT_CODE>;
        offData(
            dport: string,
            handler?: Bnrtc2DataHandler
        ): Promise<RESULT_CODE>;
    }

    type Bnrtc2DataHandler = (data: Bnrtc2Data) => void;
    interface Bnrtc2Data {
        address: string;
        dport: string;
        data: Uint8Array;
        dst: string;
        devid: string;
        isSync: boolean;
    }

    interface Channel extends WebSocket { }
    var Channel: {
        prototype: Channel;
        new(url: string, protocols?: string | string[]): Channel;
        readonly CLOSED: number;
        readonly CLOSING: number;
        readonly CONNECTING: number;
        readonly OPEN: number;
    };
    type Fetch = typeof fetch; //(input: RequestInfo, init?: RequestInit) => Promise<Response>;

    interface Global {
        Channel: typeof Channel;
        fetch: Fetch;
    }

    var bnrtc2Global: Global;
}
