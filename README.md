# ROADMAP

- [ ] 自动编译成基于 Socket 的协议接口
  > 绑定到 TCPSocket 与 WebSocket 协议上，使得其它程序可以基于相对应的协议进行连接并使用 API
  > 基于 Socket 接口在传输数据的时候，能否直接转发数据包，而不要拼接成一个 DataChannelMessage 后再发送。
- [ ] 增加 natName 验证协议
  > 需要节点绑定 BFChian 私钥，从而生成公钥与地址，并基于非对称加密进行连接验证
- [ ] ~~外部自定义 Client~~
  > 因为 Client 只需要 WebRTC+http 协议的支持，所以理论上可以在外部完整实现 client 的逻辑
- [ ] 增加 QUIC（Server+Client）的支持，与 HTTP 协议统计并存
- [ ] 使用 QUIC 替代 WebRTC 内的 SRTP/SCTP
- [ ] NatDuplex 内置 MultiStream 的支持
- [ ] 增加动态路由表的支持
  - [ ] 路由表查询
  - [ ] 路由表自动扫描与更新
- [X] 生成Android AAR