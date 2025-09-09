#!/bin/bash

# set -eou pipefail

SCALE=4000
WORKERS_LOAD=8
WORKERS_RUN=1
BATCH_SIZE=10000
NUM_QUERIES=100
DUCKDB_WRITE_PATH="tsbs.duckdb"

stamp=$(date +%Y%m%d_%H%M%S)_$(git rev-parse --short HEAD)
mkdir -p benchmark_results/"$stamp"/timescaledb
mkdir -p benchmark_results/"$stamp"/influx3
mkdir -p benchmark_results/"$stamp"/clickhouse
mkdir -p benchmark_results/"$stamp"/duckdb

generate_cpu_only_data() {
  echo "$1 | Generating data..."
  "$(pwd)"/bin/tsbs_generate_data --use-case="cpu-only" --seed=123 --scale="$SCALE" \
    --timestamp-start="2016-01-01T00:00:00Z" --timestamp-end="2016-01-04T00:00:00Z" \
    --log-interval="10s" --format="$1" \
    | gzip > /tmp/"$1"-data.gz
}

delete_cpu_only_data() {
  rm /tmp/"$1"-data.gz
}

generate_cpu_only_queries() {
  echo "$1 | Generating queries..."
  mkdir -p /tmp/"$1"_queries
  FORMATS="$1" SCALE="$SCALE" SEED=123 TS_START="2016-01-01T00:00:00Z" TS_END="2016-01-04T00:00:01Z" \
  QUERIES="$NUM_QUERIES" BULK_DATA_DIR=/tmp/"$1"_queries EXE_FILE_NAME="$(pwd)"/bin/tsbs_generate_queries \
  scripts/generate_queries.sh
}

delete_cpu_only_queries() {
  rm -rf /tmp/"$1"_queries
}

load_cpu_only_queries() {
  echo "$1 | Benchmarking ingestion..."
  target_system="$1"
  shift

  run_exec="$(pwd)"/bin/tsbs_load_"$target_system"
  cat /tmp/"$target_system"-data.gz | gunzip | "$run_exec" --workers="$WORKERS_LOAD" --batch-size="$BATCH_SIZE" "$@" \
    | tee benchmark_results/"$stamp"/"$target_system"/load.txt
}

execute_cpu_only_queries() {
  echo "$1 | Benchmarking queries..."
  target_system="$1"
  shift

  mkdir -p benchmark_results/"$stamp"/"$target_system"/queries

  run_exec="$(pwd)"/bin/tsbs_run_queries_"$target_system"
  find /tmp/"$target_system"_queries -maxdepth 1 -type l | while read -r symlink; do
      query_name=$(basename "$symlink")
      query_name_stripped="${query_name%.*}"
      queries=$(readlink -f "$symlink")
      cat "$queries" | gunzip | "$run_exec" --workers="$WORKERS_RUN" "$@" \
        | tee benchmark_results/"$stamp"/"$target_system"/queries/"$query_name_stripped".txt
  done
}

run_benchmark() {
  generate_cpu_only_data "$1"
  load_cpu_only_queries "$@"
  delete_cpu_only_data "$1"

  generate_cpu_only_queries "$1"
  execute_cpu_only_queries "$@"
  delete_cpu_only_queries "$1"
}

run_timescaledb() {
  echo "# --- TimescaleDB ------------ #"

  docker run -d --name timescale -p 5432:5432 -e POSTGRES_PASSWORD=tsbs timescale/timescaledb:latest-pg17
  sleep 10

  run_benchmark "timescaledb" --postgres="password=tsbs"

  docker stop timescale && docker rm timescale
}

run_influx3() {
  echo "# --- InfluxDB --------------- #"

  rm -rf /tmp/influx3-state
  mkdir -p /tmp/influx3-state
  docker run -d -p 8181:8181 -v /tmp/influx3-state/data:/var/lib/influxdb3/data \
    -v /tmp/influx3-state/plugins:/var/lib/influxdb3/plugins \
    --name influx3 influxdb:3-core influxdb3 serve --node-id=my-node-0 --object-store=file \
    --wal-max-write-buffer-size=50 --wal-snapshot-size=10 \
    --data-dir=/var/lib/influxdb3/data --plugin-dir=/var/lib/influxdb3/plugins
  sleep 10
  influx_resp=$(docker exec influx3 influxdb3 create token --admin)
  influx_token=$(printf '%s\n' "$influx_resp" | grep -oE 'apiv3_[A-Za-z0-9._-]+' | head -n1)
  echo "API key: $influx_token"

  run_benchmark "influx3" --urls="http://localhost:8181" --auth-token="$influx_token"

  docker stop influx3 && docker rm influx3
}

run_clickhouse() {
 echo "# --- ClickHouse ------------- #"

 docker run -d -p 8123:8123 -p 9000:9000 -e CLICKHOUSE_PASSWORD=changeme --name clickhouse \
   --ulimit nofile=262144:262144 clickhouse/clickhouse-server
 sleep 10

 run_benchmark "clickhouse" --password="changeme"

 docker stop clickhouse && docker rm clickhouse
}

run_duckdb() {
  echo "# --- DuckDB ----------------- #"

  generate_cpu_only_data "timescaledb"
  mv /tmp/timescaledb-data.gz /tmp/duckdb-data.gz
  load_cpu_only_queries "duckdb"
  delete_cpu_only_data "duckdb"

  generate_cpu_only_queries "duckdb"
  execute_cpu_only_queries "duckdb"
  delete_cpu_only_queries "duckdb"

  rm $DUCKDB_WRITE_PATH
}

make
run_timescaledb
run_influx3
run_clickhouse
run_duckdb
