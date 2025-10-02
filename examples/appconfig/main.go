// Example demonstrating how to extend Prefab's configuration system with
// application-specific configuration.
//
// This example shows three ways to configure an application:
// 1. Via WithConfigDefaults() for default values
// 2. Via YAML file (app.yaml)
// 3. Via environment variables (PF__MYAPP__*)
//
// Run with:
//   go run examples/appconfig/main.go
//
// Or with environment overrides:
//   PF__MYAPP__CACHE_REFRESH_INTERVAL=30s go run examples/appconfig/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/logging"
)

func main() {
	// 1. Set application defaults - these can be overridden by config files or env vars
	prefab.LoadConfigDefaults(map[string]interface{}{
		"myapp.name":                 "My Application",
		"myapp.cacheRefreshInterval": "5m",
		"myapp.maxRetries":           3,
		"myapp.enableFeatureX":       false,
		"myapp.allowedHosts": []string{
			"localhost",
			"example.com",
		},
	})

	// 2. Optionally load a config file
	// Uncomment this line if you create an app.yaml file
	// prefab.LoadConfigFile("./examples/appconfig/app.yaml")

	// 3. Create the server with standard options
	s := prefab.New(
		prefab.WithPort(8080),
	)

	// Print the effective configuration
	printConfig()

	// Start background jobs using the configuration
	startBackgroundJobs()

	fmt.Println("")
	fmt.Println("Application started with custom configuration!")
	fmt.Println("Visit http://localhost:8080/")
	fmt.Println("")
	fmt.Println("Try overriding config with environment variables:")
	fmt.Println("  PF__MYAPP__CACHE_REFRESH_INTERVAL=30s")
	fmt.Println("  PF__MYAPP__MAX_RETRIES=10")
	fmt.Println("  PF__MYAPP__ENABLE_FEATURE_X=true")
	fmt.Println("")
	fmt.Println("Environment variable naming:")
	fmt.Println("  - Double underscores (__) separate config levels")
	fmt.Println("  - Single underscores (_) within a segment become camelCase")
	fmt.Println("  - Example: PF__FOO_BAR__BAZ maps to fooBar.baz")
	fmt.Println("")

	if err := s.Start(); err != nil {
		log.Fatal(err)
	}
}

func printConfig() {
	fmt.Println("=== Effective Configuration ===")
	fmt.Printf("App Name:            %s\n", prefab.ConfigString("myapp.name"))
	fmt.Printf("Cache Refresh:       %v\n", prefab.ConfigDuration("myapp.cacheRefreshInterval"))
	fmt.Printf("Max Retries:         %d\n", prefab.ConfigInt("myapp.maxRetries"))
	fmt.Printf("Feature X Enabled:   %v\n", prefab.ConfigBool("myapp.enableFeatureX"))
	fmt.Printf("Allowed Hosts:       %v\n", prefab.ConfigStrings("myapp.allowedHosts"))
	fmt.Println("================================")
	fmt.Println("")
}

func startBackgroundJobs() {
	interval := prefab.ConfigDuration("myapp.cacheRefreshInterval")

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			refreshCache()
		}
	}()

	logging.Infow(context.Background(), "Background cache refresher started",
		"interval", interval)
}

func refreshCache() {
	ctx := context.Background()
	maxRetries := prefab.ConfigInt("myapp.maxRetries")

	logging.Infow(ctx, "Refreshing cache",
		"maxRetries", maxRetries,
		"featureXEnabled", prefab.ConfigBool("myapp.enableFeatureX"))

	// Your cache refresh logic here...
}
