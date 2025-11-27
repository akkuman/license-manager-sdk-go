## License Manager Go SDK

[English](README.md) | 简体中文

一个封装 License Manager 激活、校验、存储与心跳流程的 Go SDK，可同时适配在线与离线场景并尽量降低业务侵入。

### 功能亮点
- 统一的 `license.Client`，负责激活、校验、心跳与生命周期管理。
- 硬件指纹采集、存储后端均可插拔，易于自定义。
- 默认提供 AES-GCM 加密本地缓存以及 RSA 签名校验，保障许可证安全。
- 自动心跳循环，支持动态间隔与退避策略，并提供回调处理更新与异常。
- 附带 `examples/basic` 最小示例。

### 快速开始
1. 将授权码文件放在 `license_code/authorization_code.txt`（或自定义路径），并准备服务端 RSA 公钥 `license_code/rsa_public_key.pem`。
2. 填写 `config.Config`（完整参数见下文）：

```go
cfg := &config.Config{
    Server:                "https://license.example.com",
    Product:               "edge-gateway",
    Version:               "2.3.1",
    AuthorizationCodePath: "license_code/authorization_code.txt",
    PublicKeyPath:         "license_code/rsa_public_key.pem",
    // HardwareFields 支持 mac、hostname、cpu、memory（不区分大小写）
    HardwareFields: []string{"cpu", "memory"},
    // 示例：强制 10 秒心跳，方便观察日志
    HeartbeatInterval: 10 * time.Second,
}
```

3. 创建客户端并监听回调：

```go
client, err := license.NewClient(cfg, license.WithCallbacks(license.Callbacks{
    OnLicenseUpdated: func(lic *models.LicensePayload) {
        log.Printf("许可证更新，将于 %s 过期", lic.ExpiresAt)
    },
}))
defer client.Close()

if lic := client.CurrentLicense(); lic != nil {
    log.Printf("状态=%s 过期=%s", lic.Status, lic.EndDate)
    log.Printf("最大激活=%d key=%s", lic.MaxActivations, lic.LicenseKey)
    logMap("功能配置", lic.FeatureConfig)
    logMap("自定义参数", lic.CustomParameters)
    logMap("使用限制", lic.UsageLimits)
}

log.Println("心跳以 ~10 秒发送（示例等待 35 秒后退出）")
time.Sleep(35 * time.Second)

func logMap(title string, m map[string]interface{}) {
    if len(m) == 0 {
        log.Printf("%s: (无)", title)
        return
    }
    buf, _ := json.MarshalIndent(m, "", "  ")
    log.Printf("%s:\n%s", title, buf)
}
```

更多细节可参考 `examples/basic`。

### 配置项速查
- **在线模式必填**：
  - `Server`：License Manager 地址。
  - `Product`：产品标识。
  - `Version`：产品版本。
  - `AuthorizationCodePath`（或 `AuthorizationCode`）：授权码来源。
  - `PublicKeyPath`（或 `PublicKeyPEM`）：服务端 RSA 公钥。
- **离线模式**：
  - 设置 `Offline = true`，并提供 `LicenseFilePath` + `PublicKeyPath`。
- **可选参数**：
  - `BasePath`：当 API 挂载在子路径时使用。
  - `HeartbeatInterval`：覆盖默认 5 分钟心跳间隔。
  - `HTTPTimeout`：HTTP 超时时间，默认 15 秒。
  - `LogLevel`：`debug`/`info`/`warn`/`error`，默认 info。
  - `StoragePath`：许可证缓存路径（默认同 `LicenseFilePath`）。
  - `StorageSecret`：设置后本地缓存将使用 AES-GCM 加密。
  - `HardwareFields`：硬件指纹字段（`mac`、`hostname`、`cpu`、`memory`）。
  - `DeviceInfo` / `Metadata`：激活请求附加信息。
  - `HTTPHeaders`：自定义 HTTP 头。

### 项目结构
- `license/`：对外入口与生命周期管理。
- `activation/`, `heartbeat/`：调用 `/api/v1/activate` 与 `/api/v1/heartbeat`。
- `validator/`：RSA 签名、硬件绑定与过期校验。
- `hardware/`：硬件指纹提供者（默认 MAC + 主机名 hash）。
- `storage/`：本地加密存储抽象。
- `config/`：配置结构体与校验逻辑。
- `logger/`：日志接口与默认实现。
- `examples/`：使用示例。

### 开发指南
- **可选硬件字段**（通过 `Config.HardwareFields` 指定）：
  - `mac`：首个非回环网卡的 MAC 地址（默认启用）。
  - `hostname`：主机名（默认启用）。
  - `cpu`：CPU 型号信息（按操作系统尽力获取）。
  - `memory`：物理内存总量（按操作系统尽力获取）。
- 使用 `go test ./...` 确认构建通过。
- 推荐接入 `golangci-lint` 做静态检查。

