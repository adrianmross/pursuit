package oidcauth

import "testing"

func TestSignVerifyRoundTrip(t *testing.T) {
	m := &Manager{cookieKey: []byte("test-secret")}
	want := statePayload{Provider: "google", State: "s1", Nonce: "n1", Exp: 123}
	cookie, err := m.sign(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.verify(cookie)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestVerifyRejectsTamperedCookie(t *testing.T) {
	m := &Manager{cookieKey: []byte("test-secret")}
	cookie, err := m.sign(statePayload{Provider: "google", State: "s1", Nonce: "n1", Exp: 123})
	if err != nil {
		t.Fatal(err)
	}
	tampered := cookie[:len(cookie)-1] + "x"
	if _, err := m.verify(tampered); err == nil {
		t.Fatal("expected tampered cookie to be rejected")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	cookie, err := (&Manager{cookieKey: []byte("key-a")}).sign(statePayload{Provider: "google", State: "s1", Nonce: "n1", Exp: 123})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&Manager{cookieKey: []byte("key-b")}).verify(cookie); err == nil {
		t.Fatal("expected mismatched key to be rejected")
	}
}
