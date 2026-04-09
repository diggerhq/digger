.PHONY: lint test deps template package \
	ns-create deploy-platform-reference deploy-opentaco upgrade-opentaco \
	ns-create-tilt create-secrets verify

CHARTS_DIR ?= helm-charts
CHART ?= opentaco
CHART_DIR := $(CHARTS_DIR)/$(CHART)
NAMESPACE ?= opentaco
VALUES_FILE ?= values-opentaco.yaml

PLATFORM_RELEASE ?= opentaco-platform-reference
PLATFORM_CHART ?= $(CHARTS_DIR)/opentaco-platform-reference

OPENTACO_RELEASE ?= opentaco
OPENTACO_CHART ?= $(CHARTS_DIR)/opentaco

SECRETS_DIR ?= $(CHARTS_DIR)/secrets-example

lint:
	helm lint $(CHART_DIR)

test:
	@if [ -d "$(CHART_DIR)/tests" ]; then \
		helm unittest $(CHART_DIR); \
	else \
		echo "No tests directory for $(CHART)"; \
	fi

deps:
	@grep -q "^dependencies:" $(CHART_DIR)/Chart.yaml && helm dependency build $(CHART_DIR) || true

template:
	helm template $(CHART) $(CHART_DIR)

package: deps
	helm package $(CHART_DIR)

ns-create:
	kubectl create namespace $(NAMESPACE)
	kubectl create namespace traefik

ns-create-tilt:
	kubectl create namespace $(NAMESPACE)
	kubectl create namespace traefik

deploy-platform-reference:
	helm upgrade --install $(PLATFORM_RELEASE) $(PLATFORM_CHART) -n $(NAMESPACE)

create-secrets:
	kubectl create secret generic ui-secrets \
	  --from-env-file=$(SECRETS_DIR)/ui.env \
	  -n $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic taco-orchestrator-secrets \
	  --from-env-file=$(SECRETS_DIR)/digger-backend.env \
	  -n $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic statesman-secrets \
	  --from-env-file=$(SECRETS_DIR)/statesman.env \
	  -n $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic drift-secrets \
	  --from-env-file=$(SECRETS_DIR)/drift.env \
	  -n $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic taco-sidecar-secrets \
	  --from-env-file=$(SECRETS_DIR)/sidecar.env \
	  -n $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -

deploy-opentaco:
	helm upgrade --install $(OPENTACO_RELEASE) $(OPENTACO_CHART) -f $(VALUES_FILE) -n $(NAMESPACE)

upgrade-opentaco:
	helm upgrade $(OPENTACO_RELEASE) $(OPENTACO_CHART) -f $(VALUES_FILE) -n $(NAMESPACE)

verify:
	kubectl get pods -n $(NAMESPACE)
	kubectl get svc -n $(NAMESPACE)
