package testutils

import (
	"net/http"
	"reflect"
	"testing"
)

func CompareRequest(t *testing.T, expected, actual *http.Request) bool {
	t.Helper()

	if expected.Method != actual.Method {
		t.Logf("expected request method: %s, got: %s", expected.Method, actual.Method)

		return false
	}

	if expected.URL.String() != actual.URL.String() {
		t.Logf("expected URL: %s, got: %s", expected.URL.String(), actual.URL.String())

		return false
	}

	if !reflect.DeepEqual(expected.Header, actual.Header) {
		t.Logf("expected headers: %s, got: %s", expected.Header, actual.Header)

		return false
	}

	if expected.ContentLength != actual.ContentLength {
		t.Logf("expected body conent-length: %d, got: %d", expected.ContentLength, actual.ContentLength)

		return false
	}

	return true
}
