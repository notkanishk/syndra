module syndra/addons/conformance

go 1.25.0

require (
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	syndra/addons/contract v0.0.0
	syndra/addons/reference v0.0.0-00010101000000-000000000000
)

require golang.org/x/text v0.14.0 // indirect

replace (
	syndra/addons/contract => ../contract
	syndra/addons/reference => ../reference
)
