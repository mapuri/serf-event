package serf_event

import "testing"

func TestHandlerSearchExactKeyMatch(t *testing.T) {
	r := NewRouter()
	r.handlers = map[string]interface{}{
		"key": "value",
	}

	res := r.findHandlerFunc("key")
	if res == nil {
		t.Fatalf("failed to find handler")
	}

	if res.(string) != "value" {
		t.Fatalf("unexpected value received. Exptd: 'value', rrcvd: %+v", res)
	}
}

func TestHandlerSearchSubRouterKeyMatch(t *testing.T) {
	r := NewRouter()
	sr := r.NewSubRouter("prefix/")
	sr.handlers = map[string]interface{}{
		"key": "value",
	}

	res := r.findHandlerFunc("prefix/key")
	if res == nil {
		t.Fatalf("failed to find handler")
	}

	if res.(string) != "value" {
		t.Fatalf("unexpected value received. Exptd: 'value', rrcvd: %+v", res)
	}
}
