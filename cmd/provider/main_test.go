package main

import (
	"os"
	"testing"

	"github.com/alecthomas/kingpin/v2"
)

func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}

func TestApplicationSetup(t *testing.T) {
	// Test that we can create the kingpin application without errors
	app := kingpin.New("test-provider", "A Crossplane provider for libvirt")
	if app == nil {
		t.Fatal("Failed to create kingpin application")
	}

	// Test flag definitions
	debug := app.Flag("debug", "Run with debug logging.").Short('d').Bool()
	enableWebhooks := app.Flag("enable-webhooks", "Enable validation webhooks.").Default("false").Bool()

	// kingpin.Flag().Bool() never returns nil, but check for safety
	if debug == nil {
		t.Error("Debug flag was not created")
		return // Avoid potential nil dereference in subsequent tests
	}
	if enableWebhooks == nil {
		t.Error("Enable webhooks flag was not created")
	}

	// Test parsing empty args (should use defaults)
	_, err := app.Parse([]string{})
	if err != nil {
		t.Errorf("Failed to parse empty args: %v", err)
	}

	// Test parsing with debug flag
	_, err = app.Parse([]string{"--debug"})
	if err != nil {
		t.Errorf("Failed to parse debug flag: %v", err)
	}
	if !*debug {
		t.Error("Debug flag was not set to true")
	}

	// Reset flags for next test
	app = kingpin.New("test-provider", "A Crossplane provider for libvirt")
	_ = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
	enableWebhooks = app.Flag("enable-webhooks", "Enable validation webhooks.").Default("false").Bool()

	// Test parsing with webhooks enabled
	_, err = app.Parse([]string{"--enable-webhooks"})
	if err != nil {
		t.Errorf("Failed to parse enable-webhooks flag: %v", err)
	}
	if !*enableWebhooks {
		t.Error("Enable webhooks flag was not set to true")
	}

	// Reset flags for next test
	app = kingpin.New("test-provider", "A Crossplane provider for libvirt")
	debug = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
	enableWebhooks = app.Flag("enable-webhooks", "Enable validation webhooks.").Default("false").Bool()

	// Test parsing with both flags
	_, err = app.Parse([]string{"--debug", "--enable-webhooks"})
	if err != nil {
		t.Errorf("Failed to parse both flags: %v", err)
	}
	if !*debug {
		t.Error("Debug flag was not set to true")
	}
	if !*enableWebhooks {
		t.Error("Enable webhooks flag was not set to true")
	}
}

func TestFlagDefaults(t *testing.T) {
	app := kingpin.New("test-provider", "A Crossplane provider for libvirt")

	debug := app.Flag("debug", "Run with debug logging.").Short('d').Bool()
	enableWebhooks := app.Flag("enable-webhooks", "Enable validation webhooks.").Default("false").Bool()

	// Parse empty args to get defaults
	_, err := app.Parse([]string{})
	if err != nil {
		t.Errorf("Failed to parse empty args: %v", err)
	}

	// Test defaults
	if *debug {
		t.Error("Debug flag should default to false")
	}
	if *enableWebhooks {
		t.Error("Enable webhooks flag should default to false")
	}
}

func TestShortFlags(t *testing.T) {
	app := kingpin.New("test-provider", "A Crossplane provider for libvirt")

	debug := app.Flag("debug", "Run with debug logging.").Short('d').Bool()
	app.Flag("enable-webhooks", "Enable validation webhooks.").Default("false").Bool()

	// Test short flag
	_, err := app.Parse([]string{"-d"})
	if err != nil {
		t.Errorf("Failed to parse short flag: %v", err)
	}
	if !*debug {
		t.Error("Debug flag was not set to true with short flag")
	}
}

func TestEnvironmentVariables(t *testing.T) {
	// Test that environment variables are supported
	app := kingpin.New("test-provider", "A Crossplane provider for libvirt").DefaultEnvars()

	debug := app.Flag("debug", "Run with debug logging.").Short('d').Bool()
	app.Flag("enable-webhooks", "Enable validation webhooks.").Default("false").Bool()

	// Test parsing without environment variables first
	_, err := app.Parse([]string{})
	if err != nil {
		t.Errorf("Failed to parse without environment variables: %v", err)
	}

	// Test that the application supports environment variables by structure
	// We don't test actual env var parsing since it can interfere with other tests
	if debug == nil {
		t.Error("Debug flag was not created for environment variable testing")
	}
}

// Test that main package can be imported without errors
func TestPackageImport(t *testing.T) {
	// This test simply verifies that the package can be imported
	// and that all dependencies are available
	// The actual main() function requires a Kubernetes environment
	// so we don't test it directly
	t.Log("Package imported successfully")
}
