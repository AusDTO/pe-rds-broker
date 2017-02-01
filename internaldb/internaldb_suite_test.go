package internaldb_test

import (
	"testing"

	"github.com/AusDTO/pe-rds-broker/testutils"
)

func TestInternalDB(t *testing.T) {
	testutils.RunTestSuite(t, "Internal DB Suite")
}
