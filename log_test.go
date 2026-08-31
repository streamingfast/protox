package protox_test

import (
	"github.com/streamingfast/logging"
)

var zlogTest, tracerTest = logging.PackageLogger("protox_test", "github.com/streamingfast/protox/test")

func init() {
	logging.InstantiateLoggers()
}
