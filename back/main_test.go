package main

import "testing"

func TestListenAddr(t *testing.T) {
	tests := []struct {
		name string
		addr string
		host string
		port string
		std  string // platform-standard PORT
		want string
	}{
		{name: "defaults", want: "0.0.0.0:8080"},
		{name: "host and port", host: "127.0.0.1", port: "9000", want: "127.0.0.1:9000"},
		{name: "ipv6 host", host: "::1", port: "8081", want: "[::1]:8081"},
		{name: "platform PORT", std: "3000", want: "0.0.0.0:3000"},
		{name: "RUDY_PORT beats PORT", port: "9000", std: "3000", want: "0.0.0.0:9000"},
		{name: "full address overrides", addr: "192.168.1.10:7000", host: "127.0.0.1", port: "9000", want: "192.168.1.10:7000"},
		{name: "bare port address", addr: "7000", host: "127.0.0.1", want: "127.0.0.1:7000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RUDY_ADDR", tt.addr)
			t.Setenv("RUDY_HOST", tt.host)
			t.Setenv("RUDY_PORT", tt.port)
			t.Setenv("PORT", tt.std)

			if got := listenAddr(); got != tt.want {
				t.Fatalf("listenAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}
