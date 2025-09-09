import os
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
import numpy as np

# ---------- Configuration ----------
RESULTS_DIR = "benchmark_results"
QUERY_COUNT = 15  # Number of query groups
TIME_STEP = 10    # Each row represents 10 seconds

sns.set_style("whitegrid")
palette = sns.color_palette("tab10")

# ---------- Helper functions ----------
def read_load_file(filepath):
    """Parse load.txt into a DataFrame, ignoring the summary footer."""
    with open(filepath) as f:
        lines = f.readlines()

    # Keep only rows until "Summary:" line
    data_lines = []
    for line in lines:
        if line.startswith("Summary:") or line.startswith("["):
            break
        data_lines.append(line)

    # Parse only valid CSV lines
    from io import StringIO
    df = pd.read_csv(StringIO("".join(data_lines)))

    # Add relative seconds based on first timestamp
    df["time_sec"] = (df["time"] - df["time"].iloc[0])
    return df[["time_sec", "overall metric/s"]]


def read_query_file(query_path):
    """Reads a query result file and returns mean and std latency in ms."""
    with open(query_path) as f:
        for line in f:
            if line.startswith("min:"):
                parts = line.split("med:")[1].split("mean:")
                mean = float(parts[1].split("ms")[0].strip())
                stddev = float(line.split("stddev:")[1].split("ms")[0].strip())
                return mean, stddev
    return None, None


# ---------- Read data ----------
systems = os.listdir(os.path.join(RESULTS_DIR, os.listdir(RESULTS_DIR)[0]))
ingest_data = {}
query_data = {}

for system in systems:
    system_path = os.path.join(RESULTS_DIR, os.listdir(RESULTS_DIR)[0], system)

    # Load ingestion
    load_path = os.path.join(system_path, "load.txt")
    ingest_data[system] = read_load_file(load_path)

    # Load queries
    query_folder = os.path.join(system_path, "queries")
    for q_file in sorted(os.listdir(query_folder)):
        q_idx = "-".join(q_file.split("-")[1:-1])  # extract query index
        mean, stddev = read_query_file(os.path.join(query_folder, q_file))
        if q_idx not in query_data:
            query_data[q_idx] = {}
        query_data[q_idx][system] = {'mean': mean, 'std': stddev}

# ---------- Plot ingestion rate ----------
plt.figure(figsize=(10, 6))
for system, df in ingest_data.items():
    plt.plot(df['time_sec'], df['overall metric/s'], label=system, marker='o')
plt.xlabel("Time (s)")
plt.ylabel("Ingestion rate (metrics/sec)")
plt.title("Time Series Ingestion Rate | 8 Workers | 4000 Hosts * 10 Metrics, 10s Interval, 3 Days (Total Metrics: 1B)")
plt.xlim(0, 200)
plt.legend()
plt.tight_layout()
plt.savefig("ingestion_bench.png")
plt.clf()

# ---------- Plot query latencies (relative) ----------
# Convert query_data to a long DataFrame
records = []
for q_idx, systems_data in query_data.items():
    for system, vals in systems_data.items():
        if system == "duckdb":
            continue
        records.append({
            "query": q_idx,
            "system": system,
            "mean": 0 if vals["mean"] == 0 else vals["mean"] / query_data[q_idx]["duckdb"]["mean"],
            "std": vals["std"]
        })
df_queries = pd.DataFrame(records)

plt.figure(figsize=(14, 6))
sns.barplot(
    data=df_queries,
    x="query", y="mean", hue="system",
    capsize=0.1, errwidth=1.5, palette="tab10",
    #errorbar=("sd", "std")  # show stddev as error bar
)
plt.axhline(1, color="black", linestyle="dotted", label="duckdb")
plt.ylabel("Avg. Latency Relative to DuckDB (lower is better)")
plt.yscale("log")
plt.xlabel("Query type")
plt.title("Query Latency | 1 Worker | 1B Metrics (= 100M rows)")
plt.xticks(rotation=45, ha="right", rotation_mode="anchor")
plt.legend(title="System")
plt.tight_layout()
plt.savefig("latency_bench_rel.png")
plt.clf()

# ---------- Plot query latencies (absolute) ----------
# Convert query_data to a long DataFrame
records = []
for q_idx, systems_data in query_data.items():
    for system, vals in systems_data.items():
        records.append({
            "query": q_idx,
            "system": system,
            "mean": vals["mean"],
            "std": vals["std"]
        })
df_queries = pd.DataFrame(records)

plt.figure(figsize=(14, 6))
sns.barplot(
    data=df_queries,
    x="query", y="mean", hue="system",
    capsize=0.1, errwidth=1.5, palette="tab10",
    #errorbar=("sd", "std")  # show stddev as error bar
)
plt.ylabel("Avg. Latency (ms)")
plt.yscale("log")
plt.xlabel("Query type")
plt.title("Query Latency | 1 Worker | 1B Metrics (= 100M rows)")
plt.xticks(rotation=45, ha="right", rotation_mode="anchor")
plt.legend(title="System")
plt.tight_layout()
plt.savefig("latency_bench_abs.png")
plt.clf()