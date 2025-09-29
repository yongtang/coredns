+++
title = "CoreDNS-1.13.0 Release"
description = "CoreDNS-1.13.0 Release Notes."
tags = ["Release", "1.13.0", "Notes"]
release = "1.13.0"
date = "2025-10-05T00:00:00+00:00"
author = "coredns"
+++

This release improves stability and resilience by addressing core panics, shutdown handling,
and import cycle issues. It also fixes data races in the file plugin, enforces size limits in
gRPC, adjusts failover behavior in forward, and prevents deadlocks in reload, making CoreDNS
more reliable in production.

## Brought to You By

Fitz_dev
Ilya Kulakov
Ville Vesilehto
Yong Tang

## Noteworthy Changes

* core: Export timeout values in dnsserver.Server (https://github.com/coredns/coredns/pull/7497)
* core: Fix Corefile related import cycle issue (https://github.com/coredns/coredns/pull/7567)
* core: Normalize panics on invalid origins (https://github.com/coredns/coredns/pull/7563)
* core: Rely on dns.Server.ShutdownContext to gracefully stop (https://github.com/coredns/coredns/pull/7517)
* plugin/dnstap: Add bounds for plugin args (https://github.com/coredns/coredns/pull/7557)
* plugin/file: Fix data race in tree Elem.Name (https://github.com/coredns/coredns/pull/7574)
* plugin/forward: No failover to next upstream when receiving SERVFAIL or REFUSED response codes (https://github.com/coredns/coredns/pull/7458)
* plugin/grpc: Enforce DNS message size limits (https://github.com/coredns/coredns/pull/7490)
* plugin/loop: Prevent panic when ListenHosts is empty (https://github.com/coredns/coredns/pull/7565)
* plugin/loop: Avoid panic on invalid server block  (https://github.com/coredns/coredns/pull/7568)
* plugin/reload: Prevent SIGTERM/reload deadlock (https://github.com/coredns/coredns/pull/7562)
