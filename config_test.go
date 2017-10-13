package main_test

import (
	cfcommon "github.com/govau/cf-common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/AusDTO/pe-rds-broker"

	"io/ioutil"

	"github.com/AusDTO/pe-rds-broker/rdsbroker"
	"gopkg.in/yaml.v2"
)

var _ = Describe("Config", func() {
	var (
		config Config

		validConfig = Config{
			LogLevel: "DEBUG",
			RDSConfig: rdsbroker.Config{
				Region:   "rds-region",
				DBPrefix: "cf",
			},
		}
	)

	Describe("Validate", func() {
		BeforeEach(func() {
			config = validConfig
		})

		It("does not return error if all sections are valid", func() {
			err := config.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error if LogLevel is not valid", func() {
			config.LogLevel = ""

			err := config.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Must provide a non-empty LogLevel"))
		})

		It("returns error if RDS configuration is not valid", func() {
			config.RDSConfig = rdsbroker.Config{}

			err := config.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Validating RDS configuration"))
		})
	})

	Describe("LoadConfig", func() {
		var (
			configFile = "config-sample.yml"
		)

		It("parses sample config", func() {
			Expect(LoadConfig(cfcommon.NewDefaultEnvLookup(), configFile)).NotTo(BeZero())
		})

		It("parses all information in sample config", func() {
			config, err := LoadConfig(cfcommon.NewDefaultEnvLookup(), configFile)
			Expect(err).NotTo(HaveOccurred())
			configStr, err := ioutil.ReadFile(configFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(yaml.Marshal(config)).To(MatchYAML(configStr))
		})
	})
})
