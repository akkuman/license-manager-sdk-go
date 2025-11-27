## License Manager Go SDK

English | [简体中文](README.zh-CN.md)

A Go SDK that wraps the ThingsPanel License Manager activation, validation, storage and heartbeat flows. It targets both online and offline scenarios with a low-touch integration surface.

### Features
- Unified client `license.Client` orchestrating activation, validation, heartbeat and lifecycle management.
- Configurable hardware fingerprint provider and pluggable storage backends.
- Built-in AES-GCM encrypted local cache and RSA signature validation for license files.
- Automatic heartbeat loop with dynamic interval/backoff plus callbacks for update/error hooks.
- Minimal example under `examples/basic`.

### Quick Start
1. Place the authorization code file at `license_code/authorization_code.txt` (or another path) and the server RSA public key at `license_code/rsa_public_key.pem`.
2. Fill a `config.Config` (see parameters below):

```go
cfg := &config.Config{
    Server:                "https://license.example.com",
    Product:               "edge-gateway",
    Version:               "2.3.1",
    AuthorizationCodePath: "license_code/authorization_code.txt",
    PublicKeyPath:         "license_code/rsa_public_key.pem",
    // HardwareFields supports mac, hostname, cpu, memory (case-insensitive).
    HardwareFields: []string{"cpu", "memory"},
    // Demo: force 10s heartbeat so logs are easy to observe.
    HeartbeatInterval: 10 * time.Second,
}
```

3. Create the client, validate once, and rely on callbacks:

```go
client, err := license.NewClient(cfg, license.WithCallbacks(license.Callbacks{
    OnLicenseUpdated: func(lic *models.LicensePayload) {
        log.Printf("license updated, expires %s", lic.ExpiresAt)
    },
}))
defer client.Close()

if lic := client.CurrentLicense(); lic != nil {
    log.Printf("status=%s expires=%s", lic.Status, lic.EndDate)
    log.Printf("max_activations=%d key=%s", lic.MaxActivations, lic.LicenseKey)
    logMap("features_config", lic.FeatureConfig)
    logMap("custom_parameters", lic.CustomParameters)
    logMap("usage_limits", lic.UsageLimits)
}

log.Println("heartbeat running every ~10s (sleeping 35s for demo)")
time.Sleep(35 * time.Second)

func logMap(title string, m map[string]interface{}) {
    if len(m) == 0 {
        log.Printf("%s: (none)", title)
        return
    }
    buf, _ := json.MarshalIndent(m, "", "  ")
    log.Printf("%s:\n%s", title, buf)
}
```

See `examples/basic` for a runnable snippet.

### Configuration Reference
- **Required (online mode)**:
  - `Server` – License Manager base URL (schema + host).
  - `Product` – Product identifier recognized by server.
  - `Version` – Product version.
  - `AuthorizationCodePath` (or inline `AuthorizationCode`) – activation code location.
  - `PublicKeyPath` (or inline `PublicKeyPEM`) – server RSA public key for signature verification.
- **Offline mode**:
  - Set `Offline = true` and provide `LicenseFilePath` + `PublicKeyPath`.
- **Optional tuning**:
  - `BasePath` – prepend to server URL if APIs are mounted under a sub-path.
  - `HeartbeatInterval` – override server/dynamic interval (defaults to 5m).
  - `HTTPTimeout` – HTTP client timeout (default 15s).
  - `LogLevel` – `debug`, `info`, `warn`, `error` (default info).
  - `StoragePath` – local license cache path (defaults to `LicenseFilePath`).
  - `StorageSecret` – when set, enables AES-GCM encryption for local cache.
  - `HardwareFields` – select fingerprint components (`mac`, `hostname`, `cpu`, `memory`).
  - `DeviceInfo`/`Metadata` – extra JSON payloads sent during activation.
  - `HTTPHeaders` – custom headers appended to activation/heartbeat calls.

### Project Layout
- `license/` – high-level client & lifecycle orchestration.
- `activation/`, `heartbeat/` – HTTP flows for `/activate` and `/heartbeat`.
- `validator/` – RSA signature & hardware/expiry checks.
- `hardware/` – fingerprint providers (default MAC+hostname hash).
- `storage/` – encrypted file store abstraction.
- `config/` – configuration struct + validation helpers.
- `logger/` – minimal interface with default stdout logger.
- `examples/` – Quick Start style sample.

### Development
- **Hardware fields** you can select via `Config.HardwareFields`:
  - `mac` – first non-loopback MAC address (default).
  - `hostname` – OS host name (default).
  - `cpu` – CPU model string (best-effort per OS).
  - `memory` – total physical memory (best-effort per OS).
- `go test ./...` to verify build.
- `golangci-lint` recommended for style/static analysis.

