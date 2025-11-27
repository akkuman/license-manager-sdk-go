# License Manager Go SDK 设计

## 1. 设计目标
- 提供统一、低侵入的 Go SDK 以封装激活、校验、心跳、许可证管理流程。
- 兼容现有 License Manager 服务端接口，在线/离线模式对业务透明。
- 高可扩展与可测试性：硬件指纹、存储、HTTP、时间均可插拔。

## 2. 交付范围
- SDK 源码与 go.mod。
- 文档：README（部署/Quick Start/FAQ）、API 说明、设计文档。
- 示例：最小 CLI 示例、等价现有 client-demo 的进阶示例。

## 3. 目录结构（建议）
- `license/`：对外主入口，生命周期管理。
- `config/`：配置解析，支持代码、环境变量、文件三种来源。
- `activation/`：激活流程、授权码加载、许可证缓存。
- `heartbeat/`：心跳调度、动态间隔、退避策略。
- `validator/`：许可证签名验证、有效期与限制校验。
- `hardware/`：指纹采集接口、默认实现、自定义适配。
- `storage/`：许可证加解密、持久化、可插拔后端。
- `logger/`：统一日志接口与默认实现。
- `internal/httpclient/`：可替换 HTTP 层与中间件。
- `examples/`：CLI 和高级示例。

## 4. 核心模块设计
- **license.Client**
  - 组合各模块，提供 `Init/Validate/CurrentLicense/Close` 等方法。
  - 维护状态机：未激活 → 激活中 → 已授权 → 心跳中 → 需重新激活。
- **config**
  - 结构体映射所有必填/可选项。
  - 合并优先级：代码 > 环境变量 > 文件。
  - 校验：在线模式需服务端地址/公钥/授权码；离线模式需许可证文件/公钥。
- **activation**
  - 在线：读取本地缓存→校验→失效则调用 `/api/v1/activate`。
  - 离线：只校验本地文件，返回明确错误提示业务获取新证书。
  - 激活成功后写入 `storage` 并触发 license 更新回调。
- **validator**
  - 负责 RSA 签名校验、AES-GCM 解密、硬件绑定比对、功能/限制判断。
  - 对外暴露 `Validate()` 与 `CurrentLicense()`。
- **heartbeat**
  - 仅在线模式启用，默认 300s，可由服务端响应或配置覆盖。
  - 失败时指数退避（含最大间隔）、网络恢复后重置。
  - 服务端返回新许可证时自动替换并回调。
  - 支持 `Pause()`/`Resume()` 控制。
- **hardware**
  - `Provider` 接口，默认实现为“标准组合指纹”。
  - 支持配置字段选择与自定义 Provider，附加匿名化钩子。
- **storage**
  - 对象接口 `Store/Load/Delete`，默认实现使用 AES-GCM 加密文件。
  - 可扩展到自定义路径、KMS、嵌入式 KV 等。
- **logger**
  - 简单接口 `Debug/Info/Warn/Error`，默认打印到 stdout，可注入业务日志器。

## 5. 关键流程
- **初始化**
  1. 加载配置，校验必填项。
  2. 初始化 logger、hardware provider、storage、HTTP client。
  3. 加载本地许可证并调用 `validator`。
  4. 在线模式：若本地无效则触发激活。
  5. 激活成功后启动心跳调度器。
- **激活请求**
  - 组装硬件指纹、产品/版本、授权码、附加字段。
  - 收到成功响应后写入 storage，更新状态并触发 `OnLicenseUpdated`。
- **心跳循环**
  - 定时发送 heartbeat，携带许可证 key 与指纹。
  - 根据响应调整间隔，处理返回的许可证或状态。
  - 网络/服务异常：记录 `OnHeartbeatError`，指数退避，视情况触发 `OnActivationRequired`。
- **离线校验**
  - 定期或按需调用 `Validate()`，若失败返回明确错误编码，供业务决定下一步。

## 6. 配置项
- 必填：`Server`（在线模式）、`Product`、`Version`、`AuthCode` 或 `LicenseFile`、`PublicKey`。
- 可选：`HeartbeatInterval`、`HTTPTimeout`、`LogLevel`、`HardwareFields`、`CustomHTTPClient`、`StoragePath`、`Callbacks`、`OfflineMode` 开关。

## 7. 错误与回调
- 统一错误类型：区分可重试（网络、临时故障）与不可重试（授权码无效、许可证撤销）。
- 回调接口：
  - `OnLicenseUpdated(license)`
  - `OnHeartbeatError(err)`
  - `OnActivationRequired(reason)`
  - 回调在独立 goroutine 执行，确保线程安全。

## 8. 非功能性要求
- **线程安全**：使用读写锁或原子变量保护许可证与心跳状态。
- **可测试性**：通过接口注入 mock HTTP、时间、硬件、存储。
- **性能**：心跳与校验轻量；许可证加解密仅在更新时执行。
- **监控**：对外暴露关键事件与指标（激活次数、心跳失败次数等）供业务埋点记录。
