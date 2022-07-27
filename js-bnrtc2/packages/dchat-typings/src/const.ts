// @ts-ignore
export enum MessageState {
    Success = "Success",                     // 消息发送成功，已投递到对方节点应用
    Failed = "Failed",                       // 消息发送失败，如缓冲区满无法发送
    Offline = "Offline",                     // 对方不在线，无法投递
    DportConflict = "Dport Conflict",        // dport被其它应用占用
    DportUnbound = "Dport Unbound",          // dport未绑定
    ConnectFailed = "Connect Failed",        // 连接失败
    FormatError = "Format Error",            // address或dport格式错误
    Timeout = "Timeout",                     // 消息响应超时
    Closed = "Closed",                       // 已关闭
    NoAddress = "NoAddress",                 // 未绑定地址
    WSError = "WsAddress",                   // websocket通信失败
    ServiceNotReady = "Service Not Ready",   // 服务未就绪
    Unknown = "Unknown",                     // 未知错误
}


export enum LoginStatus {
    Online = "Online",                     // 上线
    Offline = "Offline",                    // 下线
}

export enum ServiceMessageType {
	ONLINE  = 0,
	OFFLINE  = 1,
	ADD_FRIENDS  = 2,
	DEL_FRIENDS  = 3,
	MESSAGE  = 4,
    KEEP_ALIVE  = 5,
    ONLINE_ACK  = 6,
}

export const DCHAT_SERVICE_CLIENT_DPORT = "dchat-service-client-dport";
export const DCHAT_SERVICE_SERVER_DPORT = "dchat-service-server-dport";

export const DCHAT_SERVICE_NAME = "dchat";
export const DCHAT_SERVICE_ADDRESS_UPDATE_INTERVAL = 300000; // 5 minutes

// DPORT 前缀约束
export const DCHAT_DPORT_PREFIX = "DCHAT-";
// DPORT 最大长度
export const DCHAT_DPORT_LEN_MAX = 64;
// ADDRESS 最大长度 @FIXME 暂时对address只约束最大长度
export const DCHAT_ADDRESS_LEN_MAX = 64;
// 消息超时时间: 默认值30秒，最大值300秒，最小值5秒
export const DCHAT_MESSAGE_TIMEOUT_DEFAULT = 30;
export const DCHAT_MESSAGE_TIMEOUT_MAX = 300;
export const DCHAT_MESSAGE_TIMEOUT_MIN = 5;