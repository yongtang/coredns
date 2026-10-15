package coremain

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
)

// loadPluginOrder reads a runtime plugin-order file. Lines may contain either
// "name" or the build-time plugin.cfg spelling "name:package/path".
//
// Only directive names matter at runtime: packages are already linked into the
// binary, and the package paths in the file do not cause any dynamic imports.
// Omitting or commenting out a plugin disables its directive for this run.
// Any plugin still named in the Corefile will be rejected during Caddy parsing.
func loadPluginOrder(path string, compiled []string) ([]string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open plugin order file %q: %w", path, err)
	}
	defer file.Close()

	allowed := make(map[string]struct{}, len(compiled))
	for _, name := range compiled {
		allowed[name] = struct{}{}
	}

	seen := make(map[string]struct{})
	order := make([]string, 0, len(compiled))
	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line, _, _ := strings.Cut(scanner.Text(), "#")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		name, packagePath, hasPath := strings.Cut(line, ":")
		name = strings.TrimSpace(name)
		if name == "" || strings.ContainsAny(name, " \t") || (hasPath && (strings.TrimSpace(packagePath) == "" || strings.Contains(packagePath, ":"))) {
			return nil, fmt.Errorf("plugin order file %q:%d: invalid plugin entry %q", path, lineNo, line)
		}
		if _, ok := allowed[name]; !ok {
			return nil, fmt.Errorf("plugin order file %q:%d: plugin %q is not compiled into this CoreDNS binary", path, lineNo, name)
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("plugin order file %q:%d: duplicate plugin %q", path, lineNo, name)
		}

		seen[name] = struct{}{}
		order = append(order, name)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read plugin order file %q: %w", path, err)
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("plugin order file %q: no enabled plugins", path)
	}
	return order, nil
}

// configurePluginOrder applies the optional runtime order before Caddy parses
// the Corefile. Without a path, the compiled directive list is left intact.
// This must not be called concurrently with server startup or reload.
func configurePluginOrder(path string) error {
	if path == "" {
		return nil
	}
	order, err := loadPluginOrder(path, dnsserver.Directives)
	if err != nil {
		return err
	}
	return dnsserver.SetDirectives(order)
}
