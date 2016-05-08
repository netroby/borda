#!/bin/sh

function die() {
  echo $*
  [[ "${BASH_SOURCE[0]}" == "${0}" ]] && exit 1
}

influx -execute 'create database lantern' || die 'Unable to create database'
influx -database lantern -execute 'CREATE RETENTION POLICY short ON lantern DURATION 1h REPLICATION 1 DEFAULT' || die 'Unable to create retention policy'
influx -database lantern -execute 'CREATE CONTINUOUS QUERY health_250ms ON lantern BEGIN SELECT sum(client_error_count) as client_error_count, sum(proxy_error_count) as proxy_error_count, max(load_avg) as load_avg INTO lantern."default".health_250ms FROM health GROUP BY time(250ms), client, proxy, client_error, proxy_error END;' || die 'Unable to create continuous query 1'
influx -database lantern -execute 'CREATE CONTINUOUS QUERY proxy_250ms ON lantern BEGIN SELECT sum(client_error_count) as client_error_count, sum(proxy_error_count) as proxy_error_count, max(load_avg) as load_avg INTO lantern."default".proxy_250ms FROM health GROUP BY time(250ms), proxy END;' || die 'Unable to create continuous query 2'
influx -execute "create user test with password 'test'" || die 'Unable to create user'
influx -execute 'grant all on lantern to test' || die 'Unable to grant access to user'
