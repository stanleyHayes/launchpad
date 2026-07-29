package platform_test

import (
	"context"
	"errors"
	"testing"

	"launchpad/internal/platform"
)

var errPingFailed = errors.New("connection refused")

func okPing(context.Context) error { return nil }

func failingPing(context.Context) error { return errPingFailed }

func staticCount(n int) func(context.Context) (int, error) {
	return func(context.Context) (int, error) { return n, nil }
}

// checksByName indexes the report so assertions do not depend on check order.
func checksByName(report platform.ReadinessReport) map[string]platform.ReadinessCheck {
	out := make(map[string]platform.ReadinessCheck, len(report.Checks))
	for _, check := range report.Checks {
		out[check.Name] = check
	}

	return out
}

func TestLaunchReadinessReportsExpectedCheckShape(t *testing.T) {
	t.Parallel()

	svc, _, _ := newPlatformService(0, 0)
	svc.WithReadiness(platform.ReadinessDeps{
		MongoPing:        okPing,
		RedisPing:        okPing,
		CountPlans:       staticCount(3),
		CountFlags:       staticCount(5),
		AppEnv:           "production",
		CORSOrigins:      []string{"https://app.launchpad.example"},
		EncryptionKeySet: true,
		AnthropicKeySet:  false,
	})

	report := svc.LaunchReadiness(context.Background())
	checks := checksByName(report)

	if len(report.Checks) != 8 {
		t.Fatalf("expected 8 checks, got %d: %+v", len(report.Checks), report.Checks)
	}

	want := map[string]string{
		"mongo reachable":           "ready",
		"redis reachable":           "ready",
		"encryption key configured": "ready",
		"app environment":           "ready",
		"cors origins configured":   "ready",
		"anthropic key configured":  "watch", // missing key degrades, never blocks
		"billing plans seeded":      "ready",
		"feature flags seeded":      "ready",
	}

	for name, status := range want {
		check, ok := checks[name]
		if !ok {
			t.Fatalf("missing check %q in %+v", name, report.Checks)
		}

		if check.Status != status {
			t.Fatalf("check %q: status = %q, want %q", name, check.Status, status)
		}

		if check.Summary == "" {
			t.Fatalf("check %q has no summary", name)
		}

		// A non-ready check must tell the operator what to do.
		if check.Status != "ready" && check.Action == "" {
			t.Fatalf("check %q is %q but has no action", name, check.Status)
		}
	}
}

func TestLaunchReadinessBlocksOnFailedSignals(t *testing.T) {
	t.Parallel()

	svc, _, _ := newPlatformService(0, 0)
	svc.WithReadiness(platform.ReadinessDeps{
		MongoPing:        failingPing,
		RedisPing:        nil, // not wired
		CountPlans:       staticCount(0),
		CountFlags:       nil, // not wired: check is skipped, not faked
		AppEnv:           "local",
		CORSOrigins:      nil,
		EncryptionKeySet: false,
		AnthropicKeySet:  true,
	})

	report := svc.LaunchReadiness(context.Background())
	checks := checksByName(report)

	want := map[string]string{
		"mongo reachable":           "blocked", // ping error
		"redis reachable":           "blocked", // never wired
		"encryption key configured": "watch",   // missing but local env
		"app environment":           "watch",   // local is not a launch candidate
		"cors origins configured":   "blocked", // none configured
		"anthropic key configured":  "ready",
		"billing plans seeded":      "blocked", // zero seeded
	}

	for name, status := range want {
		check, ok := checks[name]
		if !ok {
			t.Fatalf("missing check %q in %+v", name, report.Checks)
		}

		if check.Status != status {
			t.Fatalf("check %q: status = %q, want %q", name, check.Status, status)
		}

		// A non-ready check must tell the operator what to do.
		if check.Status != "ready" && check.Action == "" {
			t.Fatalf("check %q is %q but has no action", name, check.Status)
		}
	}

	if _, ok := checks["feature flags seeded"]; ok {
		t.Fatalf("unwired counter must skip the check, got %+v", checks["feature flags seeded"])
	}
}
