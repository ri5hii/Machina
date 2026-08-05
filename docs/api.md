# API

> See [README.md](../README.md) for the main philosophy, approach, and CLI.

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/jobs` | Submit a job |
| `GET` | `/jobs` | List all jobs |
| `GET` | `/jobs/:id` | Get job status and result |
| `GET` | `/health` | Engine health check |

## Submit a job

```
POST /jobs
{
  "type": "file_encrypt",
  "payload": {
    "folder_path": "/data/input",
    "output_path": "/data/output"
  }
}
```

Response:

```
202 Accepted
{ "id": "1234567890", "status": "pending" }
```

## Built-in Jobs

| Job | Type | What it does |
|---|---|---|
| File encrypt | `file_encrypt` | AES-256-GCM encrypts every file in a folder |
| CSV transform | `csv_transform` | Applies uppercase, lowercase, or trim to every row in a CSV |
