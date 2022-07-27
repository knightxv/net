declare namespace DCHAT {
    // CHAT dport前缀，匹配如"DCHAT-BFCHAIN"/"DCHAT-FINCHAIN"，不满足格式则拒绝发送，防止占用冲突
    const DCHAT_DPORT_PREFIX = "DCHAT-";
    // DPORT最大长度
    const DCHAT_DPORT_LEN_MAX = 32;

    type MessageState = import("./const").MessageState;
    type LoginStatus = import("./const").LoginStatus;
    type READY_STATE = import("@bfchain/bnrtc2-client-typings").READY_STATE;

    // MessageState.Success            // 消息发送成功，已投递到对方节点应用
    // MessageState.Timeout            // 消息响应超时
    // MessageState.Failed             // 消息发送失败，如缓冲区满无法发送
    // MessageState.Offline            // 对方不在线，无法投递
    // MessageState.DportConflict      // dport被其它应用占用
    // MessageState.DportFormatError   // dport格式错误
    // MessageState.AddressFormatError // address格式错误

    type MessageHandler = (address: string, dport: string, data: Uint8Array, devid?: string, src?: string, isSync?: boolean) => boolean;
    type StateChangeHandler = (state: READY_STATE) => void;
    type LoginStateChangeHandler = (addresses: string[], state: LoginStatus) => void;

    interface CHAT {
        onStateChange(handler :StateChangeHandler): void;
        // 发送消息(给所有在线多端登录节点)
        send(address /* target */: string, dport: string, data: Uint8Array, devid?: string, src?: string): Promise<MessageState>;
        // 发送消息(仅给一个登录节点)
        sendOne(address /* target */: string, dport: string, data: Uint8Array, devid?: string, src?: string): Promise<MessageState>;
        // 同步消息给(给所有在线多端登录节点)
        sync(dport: string, data: Uint8Array, src?: string): Promise<MessageState>;
        // 注册接收消息回调
        onMessage(dport: string, handler: MessageHandler): Promise<boolean>;
        // 取消接收消息回调
        offMessage(dport: string, handler: MessageHandler): Promise<boolean>;
        // 获取消息超时时间(单位: 秒)，默认值30s
        getMessageTimeout(): number;
        // 设置消息超时时间(单位: 秒)
        setMessageTimeout(timeout: number): boolean;
        // 是否在线
        isOnline(address: string): Promise<boolean>;
        // 建立通信连接
        connect(address: string): Promise<boolean>;
        // 登录本地地址
        login(address: string): Promise<MessageState>;
        // 登出本地地址
        logout(address: string): Promise<MessageState>;
        // 添加关注地址
        addFriends(address: string, friends: Array<string>): Promise<MessageState>;
        // 取消关注地址
        delFriends(address: string, friends: Array<string>): Promise<MessageState>;
        // 注册其他地址登录状态变更回调
        onLoginStatusChange(handler: LoginStateChangeHandler): void;
        // 主动关闭通道
        close(): void;
    }
}