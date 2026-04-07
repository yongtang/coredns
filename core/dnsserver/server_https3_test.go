package dnsserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin"

	"github.com/miekg/dns"
)

func testServerHTTPS3(t *testing.T, path string, validator func(*http.Request) bool) *http.Response {
	t.Helper()
	c := Config{
		Zone:                    "example.com.",
		Transport:               "https",
		TLSConfig:               &tls.Config{},
		ListenHosts:             []string{"127.0.0.1"},
		Port:                    "443",
		HTTPRequestValidateFunc: validator,
	}
	s, err := NewServerHTTPS3("127.0.0.1:443", []*Config{&c})
	if err != nil {
		t.Log(err)
		t.Fatal("could not create HTTPS3 server")
	}
	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeDNSKEY)
	buf, err := m.Pack()
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(buf))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)

	return w.Result()
}

func TestCustomHTTP3RequestValidator(t *testing.T) {
	testCases := map[string]struct {
		path      string
		expected  int
		validator func(*http.Request) bool
	}{
		"default":                     {"/dns-query", http.StatusOK, nil},
		"custom validator":            {"/b10cada", http.StatusOK, validator},
		"no validator set":            {"/adb10c", http.StatusNotFound, nil},
		"invalid path with validator": {"/helloworld", http.StatusNotFound, validator},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			res := testServerHTTPS3(t, tc.path, tc.validator)
			if res.StatusCode != tc.expected {
				t.Error("unexpected HTTP code", res.StatusCode)
			}
			res.Body.Close()
		})
	}
}

func TestNewServerHTTPS3WithCustomLimits(t *testing.T) {
	maxStreams := 50
	c := Config{
		Zone:             "example.com.",
		Transport:        "https3",
		TLSConfig:        &tls.Config{},
		ListenHosts:      []string{"127.0.0.1"},
		Port:             "443",
		MaxHTTPS3Streams: &maxStreams,
	}

	server, err := NewServerHTTPS3("127.0.0.1:443", []*Config{&c})
	if err != nil {
		t.Fatalf("NewServerHTTPS3() with custom limits failed: %v", err)
	}

	if server.maxStreams != maxStreams {
		t.Errorf("Expected maxStreams = %d, got %d", maxStreams, server.maxStreams)
	}

	expectedMaxStreams := int64(maxStreams)
	if server.quicConfig.MaxIncomingStreams != expectedMaxStreams {
		t.Errorf("Expected quicConfig.MaxIncomingStreams = %d, got %d", expectedMaxStreams, server.quicConfig.MaxIncomingStreams)
	}

	if server.quicConfig.MaxIncomingUniStreams != expectedMaxStreams {
		t.Errorf("Expected quicConfig.MaxIncomingUniStreams = %d, got %d", expectedMaxStreams, server.quicConfig.MaxIncomingUniStreams)
	}
}

func TestNewServerHTTPS3Defaults(t *testing.T) {
	c := Config{
		Zone:        "example.com.",
		Transport:   "https3",
		TLSConfig:   &tls.Config{},
		ListenHosts: []string{"127.0.0.1"},
		Port:        "443",
	}

	server, err := NewServerHTTPS3("127.0.0.1:443", []*Config{&c})
	if err != nil {
		t.Fatalf("NewServerHTTPS3() failed: %v", err)
	}

	if server.maxStreams != DefaultHTTPS3MaxStreams {
		t.Errorf("Expected default maxStreams = %d, got %d", DefaultHTTPS3MaxStreams, server.maxStreams)
	}

	expectedMaxStreams := int64(DefaultHTTPS3MaxStreams)
	if server.quicConfig.MaxIncomingStreams != expectedMaxStreams {
		t.Errorf("Expected default quicConfig.MaxIncomingStreams = %d, got %d", expectedMaxStreams, server.quicConfig.MaxIncomingStreams)
	}
}

func TestNewServerHTTPS3ZeroLimits(t *testing.T) {
	zero := 0
	c := Config{
		Zone:             "example.com.",
		Transport:        "https3",
		TLSConfig:        &tls.Config{},
		ListenHosts:      []string{"127.0.0.1"},
		Port:             "443",
		MaxHTTPS3Streams: &zero,
	}

	server, err := NewServerHTTPS3("127.0.0.1:443", []*Config{&c})
	if err != nil {
		t.Fatalf("NewServerHTTPS3() with zero limits failed: %v", err)
	}

	if server.maxStreams != 0 {
		t.Errorf("Expected maxStreams = 0, got %d", server.maxStreams)
	}
	// When maxStreams is 0, quicConfig should not set MaxIncomingStreams (uses QUIC default)
	if server.quicConfig.MaxIncomingStreams != 0 {
		t.Errorf("Expected quicConfig.MaxIncomingStreams = 0 (QUIC default), got %d", server.quicConfig.MaxIncomingStreams)
	}
}

type tsigStatusPluginHTTPS3 struct{}

func (p *tsigStatusPluginHTTPS3) ServeDNS(_ context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	switch {
	case r.IsTsig() == nil:
		m.Rcode = dns.RcodeRefused
	case w.TsigStatus() != nil:
		m.Rcode = dns.RcodeNotAuth
	default:
		m.Rcode = dns.RcodeSuccess
	}

	if err := w.WriteMsg(m); err != nil {
		return dns.RcodeServerFailure, err
	}
	return dns.RcodeSuccess, nil
}

func (p *tsigStatusPluginHTTPS3) Name() string { return "tsig_status_https3" }

func testConfigWithTSIGStatusPluginHTTPS3() *Config {
	c := &Config{
		Zone:        "example.com.",
		Transport:   "https3",
		TLSConfig:   &tls.Config{},
		ListenHosts: []string{"127.0.0.1"},
		Port:        "443",
		TsigSecret: map[string]string{
			"tsig-key.": "MTIzNA==",
		},
	}
	c.AddPlugin(func(_next plugin.Handler) plugin.Handler { return &tsigStatusPluginHTTPS3{} })
	return c
}

func testServerHTTPS3Msg(t *testing.T, cfg *Config, req *dns.Msg) *dns.Msg {
	t.Helper()

	s, err := NewServerHTTPS3("127.0.0.1:443", []*Config{cfg})
	if err != nil {
		t.Fatal("could not create HTTPS3 server:", err)
	}

	buf, err := req.Pack()
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/dns-query", bytes.NewReader(buf))
	r.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	s.ServeHTTP(w, r)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected HTTP status: got %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	m := new(dns.Msg)
	if err := m.Unpack(body); err != nil {
		t.Fatal(err)
	}
	return m
}

func forgedTSIGMsgHTTPS3() *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)

	m.Extra = append(m.Extra, &dns.TSIG{
		Hdr: dns.RR_Header{
			Name:   "bogus-key.",
			Rrtype: dns.TypeTSIG,
			Class:  dns.ClassANY,
			Ttl:    0,
		},
		Algorithm:  dns.HmacSHA256,
		TimeSigned: uint64(time.Now().Unix()),
		Fudge:      300,
		MACSize:    32,
		MAC:        strings.Repeat("00", 32),
		OrigId:     m.Id,
		Error:      dns.RcodeSuccess,
	})
	return m
}

func TestServeHTTP3RejectsUnsignedTSIGRequiredRequest(t *testing.T) {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)

	resp := testServerHTTPS3Msg(t, testConfigWithTSIGStatusPluginHTTPS3(), m)
	if resp.Rcode != dns.RcodeRefused {
		t.Fatalf("expected REFUSED for unsigned request, got %s", dns.RcodeToString[resp.Rcode])
	}
}

func TestServeHTTP3RejectsForgedTSIG(t *testing.T) {
	resp := testServerHTTPS3Msg(t, testConfigWithTSIGStatusPluginHTTPS3(), forgedTSIGMsgHTTPS3())

	if resp.Rcode != dns.RcodeNotAuth {
		t.Fatalf("expected NOTAUTH for forged TSIG, got %s", dns.RcodeToString[resp.Rcode])
	}
}
