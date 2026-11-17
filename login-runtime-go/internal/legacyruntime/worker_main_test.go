package legacyruntime

import (
	"os"
	"testing"

	"github.com/cli-proxy-api/model-quality-login-runtime/internal/jshost"
)

func TestMain(m *testing.M) {
	if handled, code := bridgeWorkerFixture(os.Args[1:], os.Stdin, os.Stdout); handled {
		os.Exit(code)
	}
	if handled, code := jshost.WorkerMain(os.Args[1:], os.Stdin, os.Stdout); handled {
		os.Exit(code)
	}
	os.Exit(m.Run())
}
