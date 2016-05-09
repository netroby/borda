#!/bin/sh

function die() {
  echo $*
  [[ "${BASH_SOURCE[0]}" == "${0}" ]] && exit 1
}

influx -execute 'create database lantern' || die 'Unable to create database'
influx -database lantern -execute 'CREATE RETENTION POLICY short ON lantern DURATION 1h REPLICATION 1 DEFAULT' || die 'Unable to create retention policy'
influx -database lantern -execute 'CREATE CONTINUOUS QUERY health_1m ON lantern BEGIN SELECT max(client_error) as client_error, sum(client_success_count) as client_success_count, sum(client_error_count) as client_error_count, sum(client_error_count) / sum(client_success_count) as client_error_rate, max(proxy_error) as proxy_error, sum(proxy_success_count) as proxy_success_count, sum(proxy_error_count) as proxy_error_count, max(load_avg) as load_avg INTO lantern."default".health_1m FROM health GROUP BY time(1m), dim_client, dim_proxy, dim_client_error, dim_proxy_error END;' || die 'Unable to create continuous query 1'
influx -database lantern -execute 'CREATE CONTINUOUS QUERY proxy_1m ON lantern BEGIN SELECT sum(client_success_count) as client_success_count, sum(client_error_count) as client_error_count, sum(client_error_count) / sum(client_success_count) as client_error_rate, sum(proxy_error_count) as proxy_error_count, sum(proxy_error_count) / sum(proxy_success_count) as proxy_success_rate, max(load_avg) as load_avg INTO lantern."default".proxy_1m FROM health GROUP BY time(1m), dim_proxy END;' || die 'Unable to create continuous query 2'
influx -database lantern -execute 'CREATE CONTINUOUS QUERY client_1m ON lantern BEGIN SELECT sum(client_success_count) as client_success_count, sum(client_error_count) as client_error_count, sum(client_error_count) / sum(client_success_count) as client_error_rate, sum(proxy_success_count) as proxy_success_count, sum(proxy_error_count) as proxy_error_count, sum(proxy_error_count) / sum(proxy_success_count) as proxy_success_rate INTO lantern."default".client_1m FROM health GROUP BY time(1m), dim_client END;' || die 'Unable to create continuous query 3'
influx -execute "create user test with password 'test'" || die 'Unable to create user'
influx -execute 'grant all on lantern to test' || die 'Unable to grant access to user'
