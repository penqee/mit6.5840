
# Lab 1
## 分布式系统  
### 特征
* 并发
* 故障的独立性
* 缺乏全局时钟
### 驱动力
* 更好的性能：低延迟，一致性
* 容错：可用性和可恢复性
* 安全
### 挑战
* 实现难度大
* 故障情况复杂
* 性能提高难度大
### 实现
* RPC通信
* 线程
* 锁
* IO并发
* 多核运行

## 图解
* https://l1zelk90cop.feishu.cn/docx/BxFxdJtP9o8fH7xS96EcmXK3nHg?doc_app_id=501&blockId=doxcnK202gL1CgbJlGZJALtYFcg&blockType=whiteboard&blockToken=Jka3wEtsThvwClbmkkxcSZ72nZe

## 实验过程
* https://l1zelk90cop.feishu.cn/docx/A2gSd2U4HoANpExnu1pcRoXNnNf?from=from_copylink

# Lab2
## 键值服务器
## 实践过程 
* https://l1zelk90cop.feishu.cn/docx/TA3rdo6gnoBcLFxBh8kcWs4cnzf?from=from_copylink
* 图解：https://l1zelk90cop.feishu.cn/docx/LjGAdECg5ofHQHxgqfhccZTznCd?openbrd=1&doc_app_id=501&blockId=doxcnJK1nBB1Lbf4siLUI3n69xf&blockType=whiteboard&blockToken=SK5PwBUwThmxKZb244lcmYcznRc#doxcnJK1nBB1Lbf4siLUI3n69xf

## Testing Distributed Systems for Linearizability
* 线性化
* 线性一致性检查
* 线性化点
* 最终一致性
* 因果一致性
* 分叉一致性
* 序列化

# Lab3
##AB
* term和index确定唯一一条日志条目
* 心跳检测：比谁更新
* 选举超时
* 共享变量加锁
* A通过比较term，B则加上对日志的相关操作
* https://l1zelk90cop.feishu.cn/docx/WnMFdqK6Jo9kiLxygrVcWrFenhh?openbrd=1&doc_app_id=501&blockId=doxcnyftGIIbeZyp34WJiuUoz6f&blockType=whiteboard&blockToken=IILewMATEhUjf3bPmn7cAgHfnPg#doxcnyftGIIbeZyp34WJiuUoz6f

## safety : 某些节点可能不可用
* election restriction ： 日志条目只会从leader流向follower。候选者的日志条目至少和集群多数中的任何其他日志一样最新才能获得选票
* Committing entries from previous terms ：存储在大多服务器上的条目并不能保证一定提交了，仍有可能会被覆盖
* Safety argument
* Follower and candidates crashes ： 处理方式比较简单，cranshed后rpc调用就会失败，重启后接受rpc请求在响应前崩溃也不会成功响应
* Timing and availability ： broadcastTime ≪ electionTimeout ≪ MTBF
* https://l1zelk90cop.feishu.cn/docx/C6TMduodHooxBIxhb6Nc8MdVnVH?from=from_copylink