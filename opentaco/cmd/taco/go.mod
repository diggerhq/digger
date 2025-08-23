module github.com/diggerhq/digger/opentaco/cmd/taco

go 1.25

require (
	github.com/diggerhq/digger/opentaco/pkg/sdk v0.0.0
	github.com/google/uuid v1.5.0
	github.com/spf13/cobra v1.8.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

replace github.com/diggerhq/digger/opentaco/pkg/sdk => ../../pkg/sdk
