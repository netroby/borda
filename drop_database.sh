#!/bin/sh

influx -execute "drop database lantern"
influx -execute "drop user test"
