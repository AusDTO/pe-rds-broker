package rdsbroker_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/AusDTO/pe-rds-broker/rdsbroker"
)

var _ = Describe("canUpdate", func() {
	var (
		oldPlan, newPlan ServicePlan
		service Service
		parameters UpdateParameters
		update bool
	)
	BeforeEach(func() {
		oldPlan.ID = "old-plan"
		newPlan.ID = "new-plan"
		service.PlanUpdateable = true
	})
	JustBeforeEach(func() {
		update = CanUpdate(oldPlan, newPlan, service, parameters)
	})

	Context("service not updatable", func() {
		BeforeEach(func() {
			service.PlanUpdateable = false
		})
		It("fails", func() {
			Expect(update).To(BeFalse())
		})
	})

	Context("changing engine", func() {
		BeforeEach(func() {
			oldPlan.RDSProperties.Engine = "one"
			newPlan.RDSProperties.Engine = "two"
		})
		It("fails", func() {
			Expect(update).To(BeFalse())
		})
	})

	Context("changing from shared", func() {
		BeforeEach(func() {
			oldPlan.RDSProperties.Shared = true
			newPlan.RDSProperties.Shared = false
		})
		It("fails", func() {
			Expect(update).To(BeFalse())
		})
	})

	Context("changing to shared", func() {
		BeforeEach(func() {
			oldPlan.RDSProperties.Shared = false
			newPlan.RDSProperties.Shared = true
		})
		It("fails", func() {
			Expect(update).To(BeFalse())
		})
	})

	Context("non-changing plan", func() {
		BeforeEach(func() {
			newPlan.ID = oldPlan.ID
		})
		It("succeeds", func() {
			Expect(update).To(BeTrue())
		})
	})
})
