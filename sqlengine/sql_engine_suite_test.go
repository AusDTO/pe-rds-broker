package sqlengine_test

import (
	"testing"

	"github.com/AusDTO/pe-rds-broker/testutils"
)

func TestSQLEngine(t *testing.T) {
	testutils.RunTestSuite(t, "SQL Engine Suite")
}
