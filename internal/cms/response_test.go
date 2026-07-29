package cms_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"launchpad/internal/cms"
)

func TestPageResponseMatchesDocumentJSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	page := cms.Page{
		ID:          "page-1",
		Slug:        "product",
		Title:       "Product",
		Summary:     "About the product",
		Body:        "Long body",
		Status:      "published",
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if got, want := marshalToMap(t, page.ToResponse()), marshalToMap(t, page); !reflect.DeepEqual(got, want) {
		t.Errorf("response JSON = %v, want %v", got, want)
	}

	empty := cms.Page{}
	if got, want := marshalToMap(t, empty.ToResponse()), marshalToMap(t, empty); !reflect.DeepEqual(got, want) {
		t.Errorf("empty response JSON = %v, want %v", got, want)
	}
}

func marshalToMap(t *testing.T, value any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	return out
}
