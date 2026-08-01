# PezMax-Agent

PezMax-Agent is an independent Go service for the first phase of PezMax intelligent agent features.

It is intentionally decoupled from the current Java backend. The agent talks to the backend through an adapter interface, so a future Go backend only needs a new adapter or compatible HTTP API.

## Phase 1 scope

- Natural-language file search
- Upload metadata suggestion
- File audit suggestion
- Basic chat entry for routing search-like requests

The service can start without an LLM API key. In that mode it uses rule-based fallbacks. When `DASHSCOPE_API_KEY` is configured, Eino is used to call a DashScope OpenAI-compatible chat model for better extraction and suggestions.

## Run

```powershell
$env:PEZMAX_BACKEND_BASE_URL="http://localhost:8080"
$env:DASHSCOPE_API_KEY="your-key"
go run ./cmd/server
```

Optional environment variables:

```text
PEZMAX_AGENT_ADDR=:8090
PEZMAX_BACKEND_BASE_URL=http://localhost:8080
PEZMAX_BACKEND_TOKEN=
PEZMAX_BACKEND_TIMEOUT_SECONDS=15
PEZMAX_LLM_BASE_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
PEZMAX_LLM_MODEL=qwen-plus
PEZMAX_LLM_TEMPERATURE=0.2
PEZMAX_LLM_MAX_TOKENS=1200
DASHSCOPE_API_KEY=
```

## APIs

### Health

```http
GET /health
```

### Natural-language file search

```http
POST /api/v1/agent/files/search
Content-Type: application/json

{
  "query": "找齐鲁工业大学 2024 高数期末试卷",
  "pageSize": 10
}
```

The Java backend adapter calls:

- `/datum/file/search`
- `/datum/file/list`

### Upload metadata suggestion

```http
POST /api/v1/agent/files/metadata/suggest
Content-Type: application/json

{
  "fileName": "齐鲁工业大学_高数_2024_期末.pdf",
  "schoolHint": "齐鲁工业大学"
}
```

The Java backend adapter can use:

- `/datum/file/schools`
- `/datum/file/subjects`

### Audit suggestion

```http
POST /api/v1/agent/files/audit/suggest
Content-Type: application/json

{
  "file": {
    "fileId": 1001,
    "fileName": "高数2024期末.pdf",
    "fileUrl": "http://minio/example.pdf",
    "fileSchool": "齐鲁工业大学",
    "fileSubject": "高数",
    "fileYear": 2024,
    "fileType": 1
  }
}
```

The agent only returns an audit suggestion. It does not approve, reject, delete, or ban users.

### Chat

```http
POST /api/v1/agent/chat
Content-Type: application/json

{
  "userId": 1,
  "message": "帮我找 2024 年高数期末资料"
}
```

Search-like chat messages are routed to the file search flow.

### File recommendations

Recommend by a seed file:

```http
POST /api/v1/agent/files/recommend
Content-Type: application/json

{
  "fileId": 964,
  "limit": 8
}
```

Recommend by filters:

```http
POST /api/v1/agent/files/recommend
Content-Type: application/json

{
  "school": "齐鲁工业大学",
  "subject": "高数",
  "year": 2024,
  "type": 1,
  "limit": 8
}
```

The response contains scored recommendations and reasons:

```json
{
  "intent": "file_recommend",
  "recommendations": [
    {
      "file": {},
      "score": 90,
      "reasons": ["subject matched", "same school as current file"]
    }
  ]
}
```

### Organize favorites

```http
POST /api/v1/agent/favorites/organize
Content-Type: application/json

{
  "userId": 10,
  "groupBy": "subject",
  "pageSize": 100
}
```

`groupBy` can be `subject`, `type`, `year`, or `school`.

The response contains grouped favorite files, group priority, and suggestions:

```json
{
  "intent": "favorites_organize",
  "total": 12,
  "groups": [
    {
      "key": "subject:高等数学",
      "label": "高等数学",
      "count": 4,
      "priority": "high",
      "items": []
    }
  ],
  "suggestions": ["review 高等数学 first; it contains exam-oriented files"]
}
```

### Summarize reports

Summarize one report:

```http
POST /api/v1/agent/reports/summarize
Content-Type: application/json

{
  "reportId": 1
}
```

Summarize pending reports for one file:

```http
POST /api/v1/agent/reports/summarize
Content-Type: application/json

{
  "fileId": 964,
  "result": "0",
  "pageSize": 20
}
```

The response contains report clues, related file metadata, audit risk, and next actions.

## Verify

```powershell
go test ./...
go run ./cmd/server
```
