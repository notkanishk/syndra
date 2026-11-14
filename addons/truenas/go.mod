module syndra/addons/truenas

go 1.27.4

require (
	github.com/gorilla/websocket v1.5.3
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	github.com/truenas/api_client_golang v0.0.0-20260827000124-508624041bb3
	go.etcd.io/bbolt v1.5.0
	syndra/addons/strictjson v0.0.0
)

require (
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/text v0.44.0 // indirect
)

replace syndra/addons/strictjson => ../strictjson
