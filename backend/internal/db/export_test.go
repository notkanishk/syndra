package db

// LiveDB hands the live-database harness to this directory's external test
// package (db_test), which can import services and cache where package db
// cannot: services wires FollowRules, and the token compiler is in cache.
var LiveDB = liveDB
