package bamboohr_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"launchpad/internal/hris"
	"launchpad/internal/hris/bamboohr"
)

func TestFetchDirectoryParsesEmployees(t *testing.T) {
	t.Parallel()

	var gotAuth bool

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		gotAuth = ok && user == "api-token" && pass == "x"
		gotPath = r.URL.Path

		_, _ = w.Write([]byte(`{"employees":[
			{"id":"1","firstName":"Jane","lastName":"Doe","workEmail":"Jane@Co.com","jobTitle":"Engineer","department":"R&D"},
			{"id":"2","firstName":"Bob","lastName":"Lee","workEmail":"bob@co.com"}
		]}`))
	}))
	defer server.Close()

	config := hris.Config{Subdomain: "acme", APIToken: "api-token", BaseURL: server.URL}

	entries, err := bamboohr.NewClient(server.Client()).FetchDirectory(context.Background(), config)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if !gotAuth {
		t.Fatalf("expected basic auth with api token")
	}

	if !strings.Contains(gotPath, "/acme/v1/employees/directory") {
		t.Fatalf("unexpected path: %s", gotPath)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Email != "jane@co.com" || entries[0].JobTitle != "Engineer" || !entries[0].Active {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
}

func TestFetchDirectoryErrorsOnNon2xx(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	config := hris.Config{Subdomain: "acme", APIToken: "bad", BaseURL: server.URL}

	if _, err := bamboohr.NewClient(server.Client()).FetchDirectory(context.Background(), config); err == nil {
		t.Fatalf("expected error on 401")
	}
}
