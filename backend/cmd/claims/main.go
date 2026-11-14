// Command claims is the claims data plane in its own container
// (claims-delivery-integrity S7, assessment P4): the Actions v2 target and its
// health on :8090, answering from the snapshot in Redis. It imports only
// claims, claimtel, claimserve and periodic, so its image changes only when
// that code does, and a backend deploy does not recreate it.
package main

import (
	// Zone data in the binary, so TZ works in a minimal image.
	_ "time/tzdata"

	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"syndra/internal/claimserve"
	"syndra/internal/logstamp"
	"syndra/internal/ratelimit"
)

func main() {
	logstamp.Install()
	healthcheck := flag.Bool("healthcheck", false, "exit 0 when this container's /healthz/claims answers, for the compose healthcheck")
	flag.Parse()
	if *healthcheck {
		os.Exit(checkHealth("http://127.0.0.1" + claimserve.Addr() + "/healthz/claims"))
	}

	if p := configProblems(os.Getenv); len(p) > 0 {
		log.Fatalf("[DATA PLANE] Refusing to start: %s.", strings.Join(p, "; "))
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := claimserve.NewRedisClient(addr)
	proc := claimserve.NewProcess(rdb, os.Getenv("ZITADEL_ACTION_SIGNING_KEY"), os.Getenv("ZITADEL_DOMAIN") == "")
	// Failed signatures only, per source; a signed call is never counted.
	proc.Limit = ratelimit.FromEnv(rdb, os.Getenv)
	srv := claimserve.NewServer(claimserve.Addr(), proc.Router())
	go func() {
		log.Printf("[DATA PLANE] listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[DATA PLANE] server: %v", err)
		}
	}()
	// Until Redis answers, every call answers degraded (counted); once it
	// does, the snapshot the write side left there (no TTL) answers at once.
	go proc.Run(ctx)

	<-ctx.Done()
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		log.Printf("[DATA PLANE] shutdown: %v", err)
	}
	proc.Flush(sctx) // the last few seconds of counts
	_ = rdb.Close()
}

// configProblems is what refuses a start: in production (ZITADEL_DOMAIN set)
// the env key is the bootstrap until the keyring is read, so it must be a
// real key.
func configProblems(getenv func(string) string) []string {
	if getenv("ZITADEL_DOMAIN") == "" {
		return nil
	}
	if p := claimserve.KeyProblem(getenv("ZITADEL_ACTION_SIGNING_KEY")); p != "" {
		return []string{"ZITADEL_ACTION_SIGNING_KEY " + p}
	}
	return nil
}

// checkHealth is the liveness the compose healthcheck reads: the door
// answers its health (200, or 503 with a status, which is still an answer).
func checkHealth(url string) int {
	c := http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable {
		return 0
	}
	return 1
}
