package main

import (
	"context"
	"fmt"
	"log"

	"mkauth/internal/cache"
	"mkauth/internal/db"
)

func main() {
	// 1. Manually setup DB connections
	db.ConnectPostgres()
	db.ConnectRedis()

	ctx := context.Background()

	// 2. Clear existing rules for testing
	_, _ = db.PG.Exec(ctx, "DELETE FROM mapping_rules")

	// 3. Test Cycle Detection
	fmt.Println("--- Testing Cycle Detection ---")
	id1, err := db.CreateMappingRule(ctx, "projectA", "admin", "projectB", "viewer")
	if err != nil {
		log.Fatalf("FAIL: Initial rule creation failed: %v", err)
	}
	fmt.Printf("Initial rule created: %s\n", id1)

	_, err = db.CreateMappingRule(ctx, "projectB", "viewer", "projectA", "admin")
	if err == nil {
		log.Fatal("FAIL: Circular dependency should have been detected (A->B, B->A)")
	}
	fmt.Println("PASS: Circular dependency detected (A->B, B->A)")

	// 4. Test Chain Logic: projectA:admin -> projectB:viewer -> projectC:auditor
	_, err = db.CreateMappingRule(ctx, "projectB", "viewer", "projectC", "auditor")
	if err != nil {
		log.Fatalf("FAIL: Chain rule creation failed: %v", err)
	}
	fmt.Println("Chain rule created (A->B, B->C)")

	// 5. Test Cache Compilation for "dev_admin"
	// dev_admin has "admin" in "projectA" by mock.
	fmt.Println("--- Testing Cache Compilation for dev_admin ---")

	// Compile cache for projectC (should have auditor)
	err = cache.CompileUserCache(ctx, "dev_admin", "projectC")
	if err != nil {
		log.Fatalf("FAIL: Cache compilation failed: %v", err)
	}

	// Verify Redis content
	val, err := db.Redis.Get(ctx, "mapping:dev_admin:projectC").Result()
	if err != nil {
		log.Fatalf("FAIL: Could not fetch Redis key: %v", err)
	}
	fmt.Printf("PASS: dev_admin cache in projectC: %s\n", val)

	// Test a user WITHOUT roles
	fmt.Println("--- Testing Cache Compilation for generic user ---")
	err = cache.CompileUserCache(ctx, "other_user", "projectC")
	if err != nil {
		log.Fatalf("FAIL: Cache compilation failed: %v", err)
	}
	val, err = db.Redis.Get(ctx, "mapping:other_user:projectC").Result()
	if err != nil {
		log.Fatalf("FAIL: Could not fetch Redis key: %v", err)
	}
	fmt.Printf("PASS: generic user cache in projectC (should be empty): %s\n", val)
}
