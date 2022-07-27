export const DEFAULT_BASE_API_HOSTNAME = "127.0.0.1";
export const DEFAULT_BASE_API_PORT = 19020;

export enum API_PATH {
    HTTP_API_PATH_SERVER_BIND_ADDRESS = "/bnrtc2/server/bindAddress",
    HTTP_API_PATH_SERVER_UNBIND_ADDRESS = "/bnrtc2/server/unbindAddress",
    HTTP_API_PATH_SERVER_IS_ONLINE = "/bnrtc2/server/isOnline",
    HTTP_API_PATH_SERVER_ADD_PEERS = "/bnrtc2/server/addPeers",
    HTTP_API_PATH_SERVER_DEL_PEERS = "/bnrtc2/server/deletePeers",
    HTTP_API_PATH_SERVER_GET_PEERS = "/bnrtc2/server/getPeers",
    HTTP_API_PATH_SERVER_GET_SERVER_CONFIG = "/bnrtc2/server/serverConfig",
    HTTP_API_PATH_SERVER_GET_SERVICE_INFO = "/bnrtc2/server/getServiceInfo",
    HTTP_API_PATH_SERVER_GET_SERVICE_ADDRESSES = "/bnrtc2/server/getServiceAddresses",
    HTTP_API_PATH_SERVER_REGIST_SERVICE = "/bnrtc2/server/registService",
    HTTP_API_PATH_SERVER_CONNECT_ADDRESS = "/bnrtc2/server/connectAddress",
    HTTP_API_PATH_SERVER_GET_ADDRESSINFO = "/bnrtc2/server/getAddressInfo",

    WS_API_PATH_SERVER_CONNECT = "/bnrtc2/server/connect",
}
