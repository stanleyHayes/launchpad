package platform

import (
	"context"
	"fmt"
	"time"
)

// readinessPingTimeout bounds each dependency ping so a hung store cannot
// stall the whole report.
const readinessPingTimeout = 2 * time.Second

// Launch-readiness check statuses.
const (
	checkStatusReady   = "ready"
	checkStatusWatch   = "watch"
	checkStatusBlocked = "blocked"
)

// ReadinessCheck is one evaluated launch signal.
type ReadinessCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	Action  string `json:"action"`
}

// ReadinessReport is the GET /platform/launch-readiness payload.
type ReadinessReport struct {
	Checks []ReadinessCheck `json:"checks"`
}

// ReadinessDeps carries the live signals LaunchReadiness evaluates. Ping and
// count functions are plain values so internal/app can adapt the Mongo/Redis
// wrappers and the billing/featureflags services without this package
// importing them; secret-bearing config arrives as booleans so the endpoint
// can never leak a key. Nil ping funcs report blocked, nil count funcs skip
// the check — the report stays honest about what was not wired.
type ReadinessDeps struct {
	MongoPing        func(ctx context.Context) error
	RedisPing        func(ctx context.Context) error
	CountPlans       func(ctx context.Context) (int, error)
	CountFlags       func(ctx context.Context) (int, error)
	AppEnv           string
	CORSOrigins      []string
	EncryptionKeySet bool
	AnthropicKeySet  bool
}

// WithReadiness attaches the launch-readiness signal sources, returning the
// service so construction sites can chain it.
func (s *Service) WithReadiness(deps ReadinessDeps) *Service {
	s.readiness = &deps

	return s
}

// LaunchReadiness evaluates the signals that decide whether this environment
// can serve a launch and reports each as ready, watch, or blocked. Status
// rules:
//   - mongo / redis ping: ready when the ping succeeds, blocked when it fails
//     or was never wired — the API cannot serve without either store.
//   - encryption key: ready when set; watch in local (development tolerates
//     it), blocked elsewhere because stored secrets cannot be encrypted.
//   - app env: ready outside local; watch for local so a dev build is never
//     mistaken for a launch candidate. Blocked when the value was not wired.
//   - cors origins: ready when at least one origin is configured, blocked
//     when empty — browser clients would be rejected outright.
//   - anthropic key: ready when set, watch when missing — the assistant falls
//     back to the offline extractive generator, degraded but not blocking.
//   - billing plans / feature flags seeded: ready when the count is above
//     zero, blocked at zero (tenants cannot be billed and flags cannot
//     resolve), watch when counting fails. Skipped when the counter was not
//     wired.
//
// Every check is reported even when others fail, and the method returns no
// error: a failed signal is a check result, not a request failure.
func (s *Service) LaunchReadiness(ctx context.Context) ReadinessReport {
	deps := ReadinessDeps{
		MongoPing:        nil,
		RedisPing:        nil,
		CountPlans:       nil,
		CountFlags:       nil,
		AppEnv:           "",
		CORSOrigins:      nil,
		EncryptionKeySet: false,
		AnthropicKeySet:  false,
	}
	if s.readiness != nil {
		deps = *s.readiness
	}

	checks := []ReadinessCheck{
		pingCheck(ctx, "mongo", deps.MongoPing),
		pingCheck(ctx, "redis", deps.RedisPing),
		encryptionKeyCheck(deps),
		appEnvCheck(deps),
		corsOriginsCheck(deps),
		anthropicKeyCheck(deps),
	}

	checks = appendSeedCheck(ctx, checks, "billing plans", deps.CountPlans)
	checks = appendSeedCheck(ctx, checks, "feature flags", deps.CountFlags)

	return ReadinessReport{Checks: checks}
}

func pingCheck(ctx context.Context, name string, ping func(context.Context) error) ReadinessCheck {
	check := ReadinessCheck{
		Name:    name + " reachable",
		Status:  checkStatusReady,
		Summary: name + " ping succeeded",
		Action:  "",
	}

	if ping == nil {
		check.Status = checkStatusBlocked
		check.Summary = name + " ping not wired into readiness deps"
		check.Action = "Wire the " + name + " ping function in internal/app"

		return check
	}

	pingCtx, cancel := context.WithTimeout(ctx, readinessPingTimeout)
	defer cancel()

	if err := ping(pingCtx); err != nil {
		check.Status = checkStatusBlocked
		check.Summary = name + " ping failed"
		check.Action = "Restore the " + name + " connection before launch"
	}

	return check
}

func encryptionKeyCheck(deps ReadinessDeps) ReadinessCheck {
	check := ReadinessCheck{
		Name:    "encryption key configured",
		Status:  checkStatusReady,
		Summary: "ENCRYPTION_KEY is set",
		Action:  "",
	}

	if deps.EncryptionKeySet {
		return check
	}

	check.Summary = "ENCRYPTION_KEY is not set"
	check.Action = "Set ENCRYPTION_KEY so stored secrets can be encrypted"
	check.Status = checkStatusBlocked

	if deps.AppEnv == "local" {
		check.Status = checkStatusWatch
	}

	return check
}

func appEnvCheck(deps ReadinessDeps) ReadinessCheck {
	check := ReadinessCheck{
		Name:    "app environment",
		Status:  checkStatusReady,
		Summary: fmt.Sprintf("APP_ENV is %q", deps.AppEnv),
		Action:  "",
	}

	switch deps.AppEnv {
	case "":
		check.Status = checkStatusBlocked
		check.Summary = "APP_ENV not wired into readiness deps"
		check.Action = "Wire cfg.AppEnv in internal/app"
	case "local":
		check.Status = checkStatusWatch
		check.Action = "Launch from a non-local build"
	}

	return check
}

func corsOriginsCheck(deps ReadinessDeps) ReadinessCheck {
	check := ReadinessCheck{
		Name:    "cors origins configured",
		Status:  checkStatusReady,
		Summary: fmt.Sprintf("%d CORS origin(s) configured", len(deps.CORSOrigins)),
		Action:  "",
	}

	if len(deps.CORSOrigins) == 0 {
		check.Status = checkStatusBlocked
		check.Summary = "no CORS origins configured"
		check.Action = "Set CORS_ORIGINS or browser clients will be rejected"
	}

	return check
}

func anthropicKeyCheck(deps ReadinessDeps) ReadinessCheck {
	check := ReadinessCheck{
		Name:    "anthropic key configured",
		Status:  checkStatusReady,
		Summary: "ANTHROPIC_API_KEY is set",
		Action:  "",
	}

	if !deps.AnthropicKeySet {
		check.Status = checkStatusWatch
		check.Summary = "ANTHROPIC_API_KEY is not set"
		check.Action = "Set ANTHROPIC_API_KEY or the assistant uses the offline extractive fallback"
	}

	return check
}

// appendSeedCheck appends a "seeded" check for a counted collection, skipping
// it entirely when the counter was not wired.
func appendSeedCheck(
	ctx context.Context,
	checks []ReadinessCheck,
	label string,
	count func(context.Context) (int, error),
) []ReadinessCheck {
	if count == nil {
		return checks
	}

	check := ReadinessCheck{Name: label + " seeded", Status: "", Summary: "", Action: ""}

	n, err := count(ctx)
	switch {
	case err != nil:
		check.Status = checkStatusWatch
		check.Summary = "could not count " + label
		check.Action = "Investigate the " + label + " store"
	case n == 0:
		check.Status = checkStatusBlocked
		check.Summary = "no " + label + " found"
		check.Action = "Run the default seeding before launch"
	default:
		check.Status = checkStatusReady
		check.Summary = fmt.Sprintf("%d %s found", n, label)
	}

	return append(checks, check)
}
