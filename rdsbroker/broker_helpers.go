package rdsbroker

import (
	"github.com/AusDTO/pe-rds-broker/internaldb"
	"fmt"
	"github.com/pivotal-cf/brokerapi"
)

func (b *RDSBroker) findObjects(instanceID string) (instance *internaldb.DBInstance, service Service, plan ServicePlan, err error) {
	var ok bool
	instance = internaldb.FindInstance(b.internalDB, instanceID)
	if instance == nil {
		err = brokerapi.ErrInstanceDoesNotExist
		return
	}

	service, ok = b.catalog.FindService(instance.ServiceID)
	if !ok {
		err = fmt.Errorf("Service '%s' not found", instance.ServiceID)
		return
	}

	plan, ok = b.catalog.FindServicePlan(instance.ServiceID, instance.PlanID)
	if !ok {
		err = fmt.Errorf("Service Plan '%s' not found", instance.PlanID)
		return
	}
	return
}

// This function does NOT cover every combination. It just picks up the obviously bad combinations.
func CanUpdate(oldPlan, newPlan ServicePlan, service Service, parameters UpdateParameters) bool {
	if !service.PlanUpdateable {
		return false
	}
	if oldPlan.ID != newPlan.ID {
		if oldPlan.RDSProperties.Shared != newPlan.RDSProperties.Shared {
			return false
		}
		if oldPlan.RDSProperties.Engine != newPlan.RDSProperties.Engine {
			return false
		}
	}
	return true
}
