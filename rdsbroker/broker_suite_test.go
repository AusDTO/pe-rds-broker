package rdsbroker_test

import (
	"testing"

	"github.com/AusDTO/pe-rds-broker/testutils"
)

func TestRDSBroker(t *testing.T) {
	testutils.RunTestSuite(t, "RDS Broker Suite")
}
