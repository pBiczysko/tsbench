# tsbench – TimescaleDB Query Benchmark CLI

`tsbench` is a Go cli tool that benchmarks the execution time of parameterised `SELECT` queries against a TimescaleDB instance. It reads query parameters from a CSV file (or standard input), dispatches jobs to configurable number of concurrent workers and emits a statistical summary upon completion. 


## Design document 
See [DESIGN.md](./docs/DESIGN.md) for:
- High‑level overview of the architecture.  
- Description of key design decisions.  
- List of assumptions.