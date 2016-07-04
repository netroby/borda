# Borda

[![wercker status](https://app.wercker.com/status/d63b521240d1cea1c2fb71061b9e3272/m "wercker status")](https://app.wercker.com/project/bykey/d63b521240d1cea1c2fb71061b9e3272)

Borda, named after
[Jean-Charles de Borda](https://en.wikipedia.org/wiki/Jean-Charles_de_Borda), is
a collection point for metrics from
[Lantern](https://github.com/getlantern/lantern) clients and servers.

## REST API
[REST API Docs](http://getlantern.github.io/borda/)

## Queries

Queries can be made using SQL sent to either port 14443 at path `/query` or
`/porcelain`.  `/porcelain` returns the same results as `/query` but in a more
machine-readable form.

Port 14443 is not open to the internet, so you need to tunnel.

```
ssh -L 14443:localhost:14443 lantern@borda.getlantern.org
```

Queries can be made using curl, like this:

```
curl -k -G "https://localhost:14443/query" --data-urlencode "=
SELECT ..."
```

Here's a full example query. Keep in mind that this is valid sql except for the
custom `ASOF` and `UNTIL` clauses.

```sql
SELECT
    SUM(error_count) / 0.01 AS error_count,
    SUM(success_count) / 0.01 AS success_count,
    SUM(error_count) / SUM(error_count + success_count) AS error_rate
FROM proxies ASOF '-1h' UNTIL '-30m'
WHERE proxy_host LIKE '188.'
GROUP BY proxy_host, period(5m)
HAVING SUM(error_rate) > 0.1
ORDER BY SUM(error_rate) DESC
LIMIT 100, 25"
```

This query does the following:

* Selects from the `proxies` table
* Selects values in the time range starting one hour in the past and ending 30 minutes in the past
* Only selects values whose `proxy_host` begins with `188.`
* Groups results into 5 minute periods
* Groups results by the `proxy_host` dimension
* For each period, calculates three different derived fields (`error_count`, `success_count` and `error_rate`) using the `SUM` aggregation operator
* Limits results to those where the calculated `error_rate` is over 0.1
* Orders the results by the `SUM` of the `error_rate` across all periods for a given `proxy_host`, in descending order
* Skips the first 25 resulting rows and returns 100 of the remaining ones
