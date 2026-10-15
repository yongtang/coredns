## CoreDNS

*coredns* - pluggable DNS nameserver optimized for service discovery and flexibility.

## Synopsis

*coredns* **[-conf FILE]** **[-plugin FILE]** **[-dns.port PORT]** **[OPTION]**...

## Description

CoreDNS is a DNS server that chains plugins. Each plugin handles a DNS feature, like rewriting
queries, kubernetes service discovery or just exporting metrics. There are many other plugins,
each described on <https://coredns.io/plugins> and their respective manual pages. Plugins not
bundled by default in CoreDNS are listed on <https://coredns.io/explugins>.

When started without options CoreDNS will look for a file named `Corefile` in the current
directory, if found, it will parse its contents and start up accordingly. If no `Corefile` is found
it will start with the *whoami* plugin (coredns-whoami(7)) and start listening on port 53 (unless
overridden with `-dns.port`).

Available options:

**-conf** **FILE**
: specify Corefile to load, if not given CoreDNS will look for a `Corefile` in the current
  directory.

**-dns.port** **PORT** or **-p** **PORT**
: override default port (53) to listen on.

**-plugin** **FILE**
: use **FILE** to choose the plugin execution order and enabled directives at startup. Each
  non-comment line contains `name` or `name:package/path` as in `plugin.cfg`. The directive
  name determines the plugin; the package path is ignored at runtime. Lines omitted or
  commented out are disabled. An unknown name, duplicate, or empty list is an error. A
  disabled plugin mentioned in the Corefile causes a startup error. Without this option,
  CoreDNS uses the plugin order compiled into the binary. This option cannot load new
  Go packages: build those plugins into the executable first.

**-pidfile** **FILE**
: write PID to **FILE**.

**-plugins**
: list all plugins and quit.

**-quiet**
: don't print any version and port information on startup.

**-version**
: show version and quit.

**-windows-service**
: run as a Windows service (only available on Windows builds, default false).

## Authors

CoreDNS Authors.

## Copyright

Apache License 2.0

## See Also

Corefile(5) @@PLUGINS@@.
