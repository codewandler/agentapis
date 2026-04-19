package integration

import (
	"os"
	"testing"
)

func skipIntegrationIfNotEnabled(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration tests in -short mode")
	}
	if os.Getenv("TEST_INTEGRATION") != "1" {
		t.Skip("set TEST_INTEGRATION=1 to run integration tests")
	}
}
