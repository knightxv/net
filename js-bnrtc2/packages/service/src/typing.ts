export enum MessageState {
    Success = "Success", // 消息发送成功，已投递到对方节点应用
    Failed = "Failed", // 消息发送失败，如缓冲区满无法发送
    Offline = "Offline", // 对方不在线，无法投递
    DportConflict = "Dport Conflict", // dport被其它应用占用
    DportUnbound = "Dport Unbound", // dport未绑定
    ConnectFailed = "Connect Failed", // 连接失败
    FormatError = "Format Error", // address或dport格式错误
    Timeout = "Timeout", // 消息响应超时
    Closed = "Closed", // 已关闭
    NoAddress = "NoAddress", // 未绑定地址
    WSError = "WsAddress", // websocket通信失败
    ServiceNotReady = "Service Not Ready", // 服务未就绪
    Unknown = "Unknown", // 未知错误
    ServiceRegistFail = "Service Regist Fail", // 未知错误
}
