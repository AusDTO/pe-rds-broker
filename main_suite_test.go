package main_test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
)

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	path := "junit.xml"
	circleReports := os.Getenv("CIRCLE_TEST_REPORTS")
	if len(circleReports) > 0 {
		os.Mkdir(fmt.Sprintf("%s/go", circleReports), 0755)
		path = fmt.Sprintf("%s/go/junit.xml", circleReports)
		junitReporter := reporters.NewJUnitReporter(path)
		RunSpecsWithDefaultAndCustomReporters(t, "Main Suite", []Reporter{junitReporter})
	} else {
		RunSpecs(t, "Main Suite")
	}
}
