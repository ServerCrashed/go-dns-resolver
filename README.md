# go-dns

A tiny UDP DNS resolver/proxy with an in-memory cache.

## Very short description

Forwards DNS queries to an upstream server (default 8.8.8.8), caches responses in memory using the query name and type as the key, and returns cached answers while honoring TTLs.

## How to run

1. Build the binary:

```bash
go build -o go-dns main.go
```

2. Run the resolver (background, logs to /tmp/go-dns.log):

```bash
./go-dns &>/tmp/go-dns.log & echo $!
```

The resolver listens on UDP port 8053 by default.

3. Query the resolver with `dig` (example):

```bash
dig @127.0.0.1 -p 8053 google.com A
```

4. Check logs for activity and cache hits:

```bash
tail -n 200 /tmp/go-dns.log
```

5. Stop the resolver (replace <pid> with the printed PID):

```bash
kill <pid>
```

## Notes

- The upstream server is hardcoded to `8.8.8.8:53` in `main.go`.
- There are no static mappings; name→IP mappings come from upstream responses which are stored in the in-memory cache.
- For development/testing only; not configured for production (no TLS, no ACLs, limited error handling).
