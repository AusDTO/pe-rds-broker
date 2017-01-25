package awsrds_test

import (
	"testing"

	"github.com/AusDTO/pe-rds-broker/testutils"
)

func TestAWSRDS(t *testing.T) {
	testutils.RunTestSuite(t, "AWS RDS Suite")
}
