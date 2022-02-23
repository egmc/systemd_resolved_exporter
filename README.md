Systemd Resolved Exporter
---

Systemd Resolved Exporter exports systemd-resolved metrics provided by `systemd-resolve --statistics` command

# usage

```
usage: systemd_resolved_exporter [<flags>]

Flags:
  -h, --help                    Show context-sensitive help (also try --help-long and --help-man).
      --listen-address=":9924"  The address to listen on for HTTP requests.
```

# sample output

```
$ curl -s  http://localhost:9924/metrics|egrep -v "go_|process|http"
# HELP systemd_resolved_cache_hits_total Total Cache Hits
# TYPE systemd_resolved_cache_hits_total counter
systemd_resolved_cache_hits_total 15877
# HELP systemd_resolved_cache_misses_total Total Cache Misses
# TYPE systemd_resolved_cache_misses_total counter
systemd_resolved_cache_misses_total 8098
# HELP systemd_resolved_current_cache_size Current Cache Size
# TYPE systemd_resolved_current_cache_size counter
systemd_resolved_current_cache_size 4
# HELP systemd_resolved_current_transactions Current Transactions
# TYPE systemd_resolved_current_transactions counter
systemd_resolved_current_transactions 0
# HELP systemd_resolved_transactions_total Total Transactions
# TYPE systemd_resolved_transactions_total counter
systemd_resolved_transactions_total 23754
```