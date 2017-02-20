#!/bin/bash
set -e

usage() {
    echo "Usage: $0 <service> <plan1> [<plan2>...]"
}

lifecycle() {
    local service=$1
    local plan=$2
    local name="$service-$plan"
    cf create-service $service $plan $name
    echo -n "waiting for create to complete..."
    while cf service $name | grep -q 'in progress'; do
        echo -n "."
        sleep 20
    done
    echo " done"
    cf create-service-key $name key
    cf service-key $name key
    cf delete-service-key -f $name key
    cf delete-service -f $name
    echo -n "waiting for delete to complete..."
    while cf service $name | grep -q 'in progress'; do
        echo -n "."
        sleep 20
    done
    echo " done"
}

SERVICE=$1
PLANS=${@:2}

if [[ -z "$SERVICE" ]] || [[ -z "$PLANS" ]]; then
    usage
    exit 1
fi

for PLAN in $PLANS; do
    lifecycle $SERVICE $PLAN
done
