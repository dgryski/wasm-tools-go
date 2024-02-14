package bindgen

import "testing"

func TestGoName(t *testing.T) {
	tests := []struct {
		name     string
		want     string
		exported string
	}{
		{"cabi", "cabi", "CABI"},
		{"datetime", "dateTime", "DateTime"},
		{"fast-api", "fastAPI", "FastAPI"},
		{"blocking-read", "blockingRead", "BlockingRead"},
		{"ipv4-socket", "ipv4Socket", "IPv4Socket"},
		{"via-ipv6", "viaIPv6", "ViaIPv6"},
		{"metadata-hash-value", "metadataHashValue", "MetadataHashValue"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GoName(tt.name, false)
			if got != tt.want {
				t.Errorf("GoName(%q, false): %q, expected %q", tt.name, got, tt.want)
			}
			exported := GoName(tt.name, true)
			if exported != tt.exported {
				t.Errorf("GoName(%q, true): %q, expected %q", tt.name, exported, tt.exported)
			}

		})
	}
}
