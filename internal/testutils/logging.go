package testutils

import (
	"flag"
	"testing"

	"github.com/brynbellomy/klog"
)

func EnableLogging(t *testing.T) {
	t.Helper()

	flag.Set("logtostderr", "true")
	flag.Set("v", "2")
	t.Cleanup(klog.Flush)
}
