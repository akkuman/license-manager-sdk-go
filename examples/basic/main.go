package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cedar-v/license-manage-sdk-go/config"
	"github.com/cedar-v/license-manage-sdk-go/license"
)

func main() {
	cfg := &config.Config{
		Server:                "http://localhost:18888",
		Product:               "demo-product",
		Version:               "1.0.0",
		AuthorizationCodePath: "license_code/authorization_code.txt",
		PublicKeyPath:         "license_code/rsa_public_key.pem",
		// HardwareFields accepts any combination of: mac, hostname, cpu, memory.
		// Here we restrict to CPU + memory only.
		HardwareFields:    []string{"cpu", "memory"},
		HeartbeatInterval: 10 * time.Second,
		Offline:           true,
	}

	client, err := license.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Validate(ctx); err != nil {
		fmt.Printf("license invalid: %v\n", err)
		return
	}
	if lic := client.CurrentLicense(); lic != nil {
		fmt.Printf("License status: %s, starts %s, expires %s\n",
			lic.Status,
			lic.StartDate.Format(time.RFC3339),
			lic.EndDate.Format(time.RFC3339))
		fmt.Printf("Deployment: %s, max activations: %d, license key: %s\n",
			lic.DeploymentType,
			lic.MaxActivations,
			lic.LicenseKey)

		printMap("Feature limits", lic.FeatureConfig)
		printMap("Custom parameters", lic.CustomParameters)
		printMap("Usage limits", lic.UsageLimits)
		printMap("Raw license payload", lic.Extras)
	}

	fmt.Println("Heartbeat running every ~10s for demo (waiting 35s before exit)...")
	time.Sleep(35 * time.Second)
	fmt.Println("Demo finished; shutting down.")
}

func printMap(title string, m map[string]interface{}) {
	if len(m) == 0 {
		fmt.Printf("%s: (none)\n", title)
		return
	}
	bytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Printf("%s: (failed to marshal: %v)\n", title, err)
		return
	}
	fmt.Printf("%s:\n%s\n", title, string(bytes))
}
