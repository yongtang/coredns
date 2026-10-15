package coremain

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/miekg/dns"
)

func TestLoadPluginOrder(t *testing.T) {
	compiled := []string{"root", "errors", "log", "cache", "forward", "whoami", "prometheus"}
	for _, tt := range []struct {
		name string
		cfg  string
		want []string
		err  string
	}{
		{
			name: "reorder, omit and comment out",
			cfg:  "# custom execution order\n forward:forward\n # cache:cache\nlog:log # enabled\nerrors:errors\n",
			want: []string{"forward", "log", "errors"},
		},
		{
			name: "accept directive names without package paths",
			cfg:  "cache\nforward\n",
			want: []string{"cache", "forward"},
		},
		{
			name: "accept plugin directive alias",
			cfg:  "prometheus:metrics\n",
			want: []string{"prometheus"},
		},
		{
			name: "missing compiled plugin",
			cfg:  "notcompiled:github.com/someone/newplugin\n",
			err:  `not compiled into this CoreDNS binary`,
		},
		{
			name: "duplicate directive",
			cfg:  "cache:cache\ncache:cache\n",
			err:  `duplicate plugin "cache"`,
		},
		{
			name: "empty list",
			cfg:  "# every plugin skipped\n\n",
			err:  "no enabled plugins",
		},
		{
			name: "empty package path",
			cfg:  "cache:\n",
			err:  "invalid plugin entry",
		},
		{
			name: "extra colon",
			cfg:  "cache:cache:another\n",
			err:  "invalid plugin entry",
		},
		{
			name: "missing directive name",
			cfg:  ":cache\n",
			err:  "invalid plugin entry",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "plugin.cfg")
			if err := os.WriteFile(file, []byte(tt.cfg), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := loadPluginOrder(file, compiled)
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("loadPluginOrder() error = %v, want %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("order = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadPluginOrderMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.cfg")
	if _, err := loadPluginOrder(path, []string{"cache"}); err == nil || !strings.Contains(err.Error(), "open plugin order file") {
		t.Fatalf("want file-open error, got %v", err)
	}
}

// These test-only plugins record the actual DNS request path, not just the
// parsed directive list. The last one answers the query, so the test needs no
// external DNS upstream, privileged port, or extra binaries such as dig.
const (
	runtimeProbeA   = "test_runtime_a"
	runtimeProbeB   = "test_runtime_b"
	runtimeProbeEnd = "test_runtime_end"
)

var (
	registerRuntimeProbes sync.Once
	runtimeProbeTrace     = struct {
		sync.Mutex
		calls []string
	}{}
)

func registerPluginOrderTestProbes() {
	registerRuntimeProbes.Do(func() {
		for _, name := range []string{runtimeProbeA, runtimeProbeB, runtimeProbeEnd} {
			plugin.Register(name, runtimeProbeSetup(name))
		}
	})
}

func runtimeProbeSetup(name string) caddy.SetupFunc {
	return func(c *caddy.Controller) error {
		for c.Next() {
			if len(c.RemainingArgs()) != 0 {
				return c.ArgErr()
			}
		}
		dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
			return runtimeProbe{name: name, next: next}
		})
		return nil
	}
}

type runtimeProbe struct {
	name string
	next plugin.Handler
}

func (p runtimeProbe) Name() string { return p.name }

func (p runtimeProbe) ServeDNS(ctx context.Context, w dns.ResponseWriter, query *dns.Msg) (int, error) {
	runtimeProbeTrace.Lock()
	runtimeProbeTrace.calls = append(runtimeProbeTrace.calls, p.name)
	runtimeProbeTrace.Unlock()

	if p.next != nil {
		return plugin.NextOrFailure(p.name, p.next, ctx, w, query)
	}

	// The final plugin terminates the chain and makes this a real DNS test.
	if p.name != runtimeProbeEnd {
		return dns.RcodeServerFailure, errors.New("unexpected end of plugin chain")
	}
	answer := new(dns.Msg)
	answer.SetReply(query)
	if err := w.WriteMsg(answer); err != nil {
		return dns.RcodeServerFailure, err
	}
	return dns.RcodeSuccess, nil
}

func resetRuntimeProbeTrace() {
	runtimeProbeTrace.Lock()
	runtimeProbeTrace.calls = nil
	runtimeProbeTrace.Unlock()
}

func readRuntimeProbeTrace() []string {
	runtimeProbeTrace.Lock()
	defer runtimeProbeTrace.Unlock()
	return slices.Clone(runtimeProbeTrace.calls)
}

func runtimePluginOrderTestSetup(t *testing.T) []string {
	t.Helper()
	registerPluginOrderTestProbes()
	original, oldCaddyQuiet, oldDNSQuiet := dnsserver.Directives, caddy.Quiet, dnsserver.Quiet
	t.Cleanup(func() {
		dnsserver.Directives, caddy.Quiet, dnsserver.Quiet = original, oldCaddyQuiet, oldDNSQuiet
	})
	caddy.Quiet, dnsserver.Quiet = true, true
	return []string{runtimeProbeA, runtimeProbeB, runtimeProbeEnd}
}

func writeRuntimePluginOrder(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime-plugins.cfg")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runtimeProbeCorefile(directives ...string) caddy.CaddyfileInput {
	return caddy.CaddyfileInput{
		Filepath:       "Corefile",
		Contents:       []byte(".:0 {\n\t" + strings.Join(directives, "\n\t") + "\n}\n"),
		ServerTypeName: "dns",
	}
}

// This checks the real request execution order. Even though the Corefile lists
// end, B, A, the compiled order (or --plugin file) decides the DNS plugin chain.
func TestRuntimePluginOrderDNSIntegration(t *testing.T) {
	compiled := runtimePluginOrderTestSetup(t)
	if fl := flag.Lookup("plugin"); fl == nil || fl.DefValue != "" {
		t.Fatalf("-plugin flag should exist and default to empty: %v", fl)
	}

	for _, tc := range []struct {
		name         string
		pluginConfig string // empty means the option was omitted
		corefile     []string
		want         []string
	}{
		{
			name:     "no flag keeps compiled order",
			corefile: []string{runtimeProbeEnd, runtimeProbeB, runtimeProbeA},
			want:     []string{runtimeProbeA, runtimeProbeB, runtimeProbeEnd},
		},
		{
			name:         "flag reverses request execution order",
			pluginConfig: runtimeProbeB + "\n" + runtimeProbeA + "\n" + runtimeProbeEnd + "\n",
			corefile:     []string{runtimeProbeEnd, runtimeProbeA, runtimeProbeB},
			want:         []string{runtimeProbeB, runtimeProbeA, runtimeProbeEnd},
		},
		{
			name:         "flag skips plugin omitted from file",
			pluginConfig: runtimeProbeB + "\n# " + runtimeProbeA + " is disabled\n" + runtimeProbeEnd + "\n",
			corefile:     []string{runtimeProbeEnd, runtimeProbeB},
			want:         []string{runtimeProbeB, runtimeProbeEnd},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := dnsserver.SetDirectives(compiled); err != nil {
				t.Fatal(err)
			}
			path := ""
			if tc.pluginConfig != "" {
				path = writeRuntimePluginOrder(t, tc.pluginConfig)
			}
			if err := configurePluginOrder(path); err != nil {
				t.Fatal(err)
			}
			if got := caddy.ValidDirectives("dns"); !slices.Equal(got, tc.want) {
				t.Fatalf("Caddy directive order = %v, want %v", got, tc.want)
			}

			resetRuntimeProbeTrace()
			instance, err := caddy.Start(runtimeProbeCorefile(tc.corefile...))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := errors.Join(instance.ShutdownCallbacks()...); err != nil {
					t.Errorf("shutdown callbacks: %v", err)
				}
				if err := instance.Stop(); err != nil {
					t.Errorf("stop: %v", err)
				}
				instance.Wait()
			})
			if len(instance.Servers()) != 1 {
				t.Fatalf("got %d DNS servers, want 1", len(instance.Servers()))
			}

			query := new(dns.Msg)
			query.SetQuestion("runtime-order.example.", dns.TypeA)
			client := &dns.Client{Net: "udp", Timeout: 2 * time.Second}
			response, _, err := client.Exchange(query, instance.Servers()[0].LocalAddr().String())
			if err != nil {
				t.Fatal(err)
			}
			if response.Rcode != dns.RcodeSuccess {
				t.Fatalf("DNS rcode = %d, want NOERROR", response.Rcode)
			}
			if got := readRuntimeProbeTrace(); !slices.Equal(got, tc.want) {
				t.Fatalf("executed plugin chain = %v, want %v", got, tc.want)
			}
		})
	}
}

// An omitted plugin must not silently run or silently ignore a Corefile block
// that still refers to it: startup should reject the unknown directive.
func TestRuntimePluginOrderDisabledDirectiveRejected(t *testing.T) {
	compiled := runtimePluginOrderTestSetup(t)
	if err := dnsserver.SetDirectives(compiled); err != nil {
		t.Fatal(err)
	}
	cfg := writeRuntimePluginOrder(t, runtimeProbeB+"\n"+runtimeProbeEnd+"\n")
	if err := configurePluginOrder(cfg); err != nil {
		t.Fatal(err)
	}
	instance, err := caddy.Start(runtimeProbeCorefile(runtimeProbeA, runtimeProbeB, runtimeProbeEnd))
	if err == nil {
		if instance != nil {
			_ = instance.Stop()
			instance.Wait()
		}
		t.Fatal("expected startup to reject a directive disabled in the --plugin file")
	}
	if !strings.Contains(err.Error(), "Unknown directive '"+runtimeProbeA+"'") {
		t.Fatalf("unexpected startup error: %v", err)
	}
}

// A bad runtime configuration must not corrupt the default compiled list.
func TestRuntimePluginOrderInvalidFileDoesNotChangeDefaults(t *testing.T) {
	compiled := runtimePluginOrderTestSetup(t)
	if err := dnsserver.SetDirectives(compiled); err != nil {
		t.Fatal(err)
	}
	file := writeRuntimePluginOrder(t, runtimeProbeA+"\nunknown_plugin:unavailable\n")
	if err := configurePluginOrder(file); err == nil {
		t.Fatal("expected uncompiled plugin to be rejected")
	}
	if got := dnsserver.Directives; !slices.Equal(got, compiled) {
		t.Fatalf("invalid runtime configuration changed directives: %v, want %v", got, compiled)
	}
}
