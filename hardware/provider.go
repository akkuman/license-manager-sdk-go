package hardware

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"net"
)

// Provider abstracts hardware fingerprint collection.
type Provider interface {
	Fingerprint(ctx context.Context) (string, map[string]string, error)
}

// DefaultProvider collects host-agnostic identifiers.
// Supported field names (case-insensitive):
//   - mac: first non-loopback MAC address (default)
//   - hostname: OS host name (default)
//   - cpu: CPU model / brand string (best-effort per OS)
//   - memory: total physical memory capacity (best-effort per OS)
//
// Extend this list by adding more cases inside Fingerprint().
type DefaultProvider struct {
	fields []string
}

// NewDefaultProvider creates a default provider selecting specific fields.
func NewDefaultProvider(fields []string) *DefaultProvider {
	var normalized []string
	for _, f := range fields {
		if strings.TrimSpace(f) != "" {
			normalized = append(normalized, strings.ToLower(strings.TrimSpace(f)))
		}
	}
	if len(normalized) == 0 {
		normalized = []string{"mac", "hostname"}
	}
	return &DefaultProvider{fields: normalized}
}

// Fingerprint gathers data and returns a deterministic hash.
func (p *DefaultProvider) Fingerprint(ctx context.Context) (string, map[string]string, error) {
	data := map[string]string{}
	for _, field := range p.fields {
		switch field {
		case "hostname":
			if host, err := os.Hostname(); err == nil {
				data["hostname"] = host
			}
		case "mac":
			if mac := firstMAC(); mac != "" {
				data["mac"] = mac
			}
		case "cpu":
			if cpu := cpuSignature(ctx); cpu != "" {
				data["cpu"] = cpu
			}
		case "memory":
			if mem := memoryCapacity(ctx); mem != "" {
				data["memory"] = mem
			}
		}
	}
	// Deterministic serialization.
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	builder := strings.Builder{}
	for _, k := range keys {
		builder.WriteString(k)
		builder.WriteString("=")
		builder.WriteString(strings.ToLower(data[k]))
		builder.WriteString(";")
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:]), data, nil
}

func firstMAC() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		mac := iface.HardwareAddr.String()
		if mac != "" {
			return strings.ToLower(mac)
		}
	}
	return ""
}

func cpuSignature(ctx context.Context) string {
	switch runtime.GOOS {
	case "linux":
		if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(strings.ToLower(line), "model name") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						return strings.TrimSpace(parts[1])
					}
				}
			}
		}
	case "darwin":
		if out := commandOutput(ctx, "sysctl", "-n", "machdep.cpu.brand_string"); out != "" {
			return out
		}
	case "windows":
		if out := commandOutput(ctx, "wmic", "cpu", "get", "Name"); out != "" {
			return out
		}
		if env := os.Getenv("PROCESSOR_IDENTIFIER"); env != "" {
			return env
		}
	}
	if out := commandOutput(ctx, "uname", "-m"); out != "" {
		return out
	}
	return runtime.GOARCH
}

func memoryCapacity(ctx context.Context) string {
	switch runtime.GOOS {
	case "linux":
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(strings.ToLower(line), "memtotal") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if value, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
							// value is in kB
							return formatBytes(value * 1024)
						}
					}
				}
			}
		}
	case "darwin":
		if out := commandOutput(ctx, "sysctl", "-n", "hw.memsize"); out != "" {
			if value, err := strconv.ParseUint(out, 10, 64); err == nil {
				return formatBytes(value)
			}
		}
	case "windows":
		if out := commandOutput(ctx, "wmic", "computersystem", "get", "TotalPhysicalMemory"); out != "" {
			if value, err := strconv.ParseUint(out, 10, 64); err == nil {
				return formatBytes(value)
			}
		}
	}
	return ""
}

func commandOutput(ctx context.Context, name string, args ...string) string {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		low := strings.ToLower(trimmed)
		if strings.Contains(low, "name") || strings.Contains(low, "totalphysicalmemory") {
			continue
		}
		return trimmed
	}
	return ""
}

func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return strconv.FormatUint(b/GB, 10) + "GB"
	case b >= MB:
		return strconv.FormatUint(b/MB, 10) + "MB"
	case b >= KB:
		return strconv.FormatUint(b/KB, 10) + "KB"
	default:
		return strconv.FormatUint(b, 10) + "B"
	}
}
