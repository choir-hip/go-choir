package proxy

import (
	"os"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
)

func TestMain(m *testing.M) { os.Exit(dbxtest.Main(m)) }
