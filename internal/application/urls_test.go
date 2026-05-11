package application

import "testing"

func TestURLBuilder_TrimsTrailingSlash(t *testing.T) {
	cases := []struct {
		name string
		base string
		want string
	}{
		{name: "no slash", base: "http://localhost:8080", want: "http://localhost:8080/api/confirm/abc"},
		{name: "single slash", base: "http://localhost:8080/", want: "http://localhost:8080/api/confirm/abc"},
		{name: "many slashes", base: "http://localhost:8080///", want: "http://localhost:8080/api/confirm/abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewURLBuilder(tc.base).Confirm("abc")
			if got != tc.want {
				t.Errorf("Confirm: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestURLBuilder_EscapesToken(t *testing.T) {
	got := NewURLBuilder("http://localhost:8080").Unsubscribe("ab/cd ef")
	want := "http://localhost:8080/api/unsubscribe/ab%2Fcd%20ef"
	if got != want {
		t.Errorf("Unsubscribe: got %q, want %q", got, want)
	}
}

func TestURLBuilder_BothEndpoints(t *testing.T) {
	b := NewURLBuilder("http://example.test")
	if c := b.Confirm("tok"); c != "http://example.test/api/confirm/tok" {
		t.Errorf("Confirm: %q", c)
	}
	if u := b.Unsubscribe("tok"); u != "http://example.test/api/unsubscribe/tok" {
		t.Errorf("Unsubscribe: %q", u)
	}
}
