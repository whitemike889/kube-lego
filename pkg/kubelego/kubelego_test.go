package kubelego

import (
	"os"
	"testing"
)

func TestKubeLego_LegoUrl(t *testing.T) {
	kl := New("")
	os.Setenv("LEGO_EMAIL", "test@example.com")
	os.Setenv("LEGO_POD_IP", "10.0.0.1")

	kl.paramsLego()
	if len(kl.LegoURL()) == 0 {
		t.Error("LegoUrl must not be empty when not set")
	}

	testurl := "http://example.com/acme"
	os.Setenv("LEGO_URL", testurl)
	kl.paramsLego()
	if kl.legoURL != testurl {
		t.Error("LegoUrl was not set correctly")
	}

	testurlWithTrailingSlash := testurl + "/"
	os.Setenv("LEGO_URL", testurlWithTrailingSlash)
	kl.paramsLego()
	if kl.legoURL != testurl {
		t.Error("Trailing slash was not removed from LegoUrl ")
	}
}
