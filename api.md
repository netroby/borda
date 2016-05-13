FORMAT: 1A
HOST: https://borda.getlantern.org

# borda

borda is an API for publishing measurements to Lantern's influxdb measurements
repository. One can domain front to borda using https://d157vud77ygy87.cloudfront.net.

## Group Measurement

## Measurements [/measurements]

### Publish a measurement [POST]

This publishes a measurement.

+ name (string) - The name of the measurement, e.g. "client_results"
+ ts (string) - The timestamp of the measurement as an ISO8601 string, e.g. "2007-04-05T14:30:25.711725Z"
+ fields (map) - A map of measurement fields. Values can be numbers, booleans or strings.

+ Request

  + Headers

            Content-Type: application/json

  + Body

            {
                "name": "client_results",
                "ts": "2007-04-05T14:30:25.711725Z",
                "fields": {
                  "client": "32DFS324DSFDSF",
                  "proxy": "185.234.23.2",
                  "user-agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/50.0.2661.94 Safari/537.36",
                  "os": "Windows",
                  "os_version": 10,
                  "lantern_version": "2.2.0 (20160413.044024)",
                  "num_errors": 5,
                  "num_successes": 987
                }
            }

+ Response 201 (text/plain)
+ Response 400 (text/plain)

            Message will indicate what specifically was wrong

+ Response 405 (text/plain)
+ Response 415 (text/plain)
