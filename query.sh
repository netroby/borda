#!/bin/sh

function die() {
  echo $*
  [[ "${BASH_SOURCE[0]}" == "${0}" ]] && exit 1
}

function query() {
  echo ""
  echo "******** $1 ********"
  echo ""
  echo "$2"
  echo ""
  influx -database lantern -execute "$2" || die "Unable to run query $2"
  echo ""
}

query "This query shows the raw data" \
      'select * from health'

query "This query shows the raw data downsampled to 250ms" \
      'select * from "default".health_250ms'

query "This query shows the downsampled data grouped by client. Notice how it becomes possible to correlate client and proxy errors" \
      'select * from "default".health_250ms group by client'

query "This query shows how to capture aggregate data per proxy" \
      'select * from "default".proxy_250ms'
