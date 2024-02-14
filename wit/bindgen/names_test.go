package bindgen

import "testing"

func TestGoName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"cabi", "CABI"},
		{"datetime", "DateTime"},
		{"fast-api", "FastAPI"},
		{"blocking-read", "BlockingRead"},
		{"ipv4-socket", "IPv4Socket"},
		{"via-ipv6", "ViaIPv6"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GoName(tt.name)
			if got != tt.want {
				t.Errorf("GoName(%q): %q, expected %q", tt.name, got, tt.want)
			}
		})
	}
}
