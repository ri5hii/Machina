# Configuration

> See [README.md](../README.md) for the main philosophy, approach, and CLI.

Machina reads defaults from `config.json`. For commands that accept `--port`, precedence is:

```
flag override -> config.json -> 8080 fallback
```

At startup, `machina start` also accepts environment overrides.

| Flag | Env | Default | Description |
|---|---|---|---|
| `--port` | `PORT` | `8080` | HTTP listen port |
| `--workers` | `WORKER_COUNT` | `4` | Number of worker goroutines |
| `--queue-size` | `QUEUE_SIZE` | `100` | Maximum queued jobs |
| `--log-level` | `LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARN`, `ERROR` |
