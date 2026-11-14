module syndra/addons/truenas

go 1.25.0

require (
	github.com/gorilla/websocket v1.5.3
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	github.com/truenas/api_client_golang v0.0.0-20250820184128-fc6edc0b6ebe
	go.etcd.io/bbolt v1.5.0
	syndra/addons/contract v0.0.0
)

require (
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace syndra/addons/contract => ../contract
