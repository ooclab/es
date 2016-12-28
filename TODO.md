# TODO

## Link

Link 现在的管理还是一团乱，有时间需要研究：

1. io.Reader 机制转发 message payload 是否可以提高效率?
2. 各种关闭、在线、离线、heartbeat等机制简单
3. Link, TunnelManager 和 conn 完全独立，有 conn 就转发消息，无则等待
4. Link 实现 Write, Read 方法，可以使用 io.Copy 与 conn 关联

## 支持

### 支持 Link on UDP

参考: kcp

### 支持 TCP on HTTP

参考：

- https://github.com/jpillora/chisel
- https://github.com/q3k/crowbar
