package testutils

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
)

func RunTestSuite(t *testing.T, name string) {
	RegisterFailHandler(Fail)
	circleReports := os.Getenv("CIRCLE_TEST_REPORTS")
	if len(circleReports) > 0 {
		os.Mkdir(fmt.Sprintf("%s/%s", circleReports, name), 0755)
		path := fmt.Sprintf("%s/%s/junit.xml", circleReports, name)
		junitReporter := reporters.NewJUnitReporter(path)
		RunSpecsWithDefaultAndCustomReporters(t, name, []Reporter{junitReporter})
	} else {
		RunSpecs(t, name)
	}
}