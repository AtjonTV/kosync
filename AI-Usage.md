# AI Usage

Sections of files and some files in whole where created by an AI Agent, LLM or similar.  
For transparency, below is a table with the files and how such an Agent interacted with it:

| File                                       | Interaction        | Agemt (Model)                           |
|--------------------------------------------|--------------------|-----------------------------------------|
| pkg/jmp-client-js                          | Writen completely  | OpenCode (Ministral 3 - 3B Ollama)      |
| webui/src/api.ts                           | Modified           | OpenCode (Ministral 3 - 3B Ollama)      |
| webui/src/stores/sync.ts                   | Modified           | OpenCode (Ministral 3 - 3B Ollama)      |
| internal/kosync/migrations_test.go         | Modified           | OpenCode (Ministral 3 - 3B Ollama)      |
| internal/kosync/crypto_test.go             | Written partially  | OpenCode (Ministral 3 - 3B Ollama)      |
| internal/kosync/database_document_test.go  | Written partially  | OpenCode (Ministral 3 - 3B Ollama)      |
| internal/kosync/database_migration_test.go | Written partially  | OpenCode (Ministral 3 - 3B Ollama)      |
| internal/kosync/koreader_test.go           | Written partially  | OpenCode (Ministral 3 - 3B Ollama)      |
| pkg/**/*_test.go                           | Modified           | OpenCode (Ministral 3 - 3B Ollama)      |
| internal/kosync/config_test.go             | Written completely | JetBrains Junie (Google Gemini 3 Flash) |
| internal/kosync/database_backup_test.go    | Written completely | JetBrains Junie (Google Gemini 3 Flash) |
| internal/kosync/database_user_test.go      | Written completely | JetBrains Junie (Google Gemini 3 Flash) |
| internal/kosync/log_test.go                | Written completely | JetBrains Junie (Google Gemini 3 Flash) |
| internal/kosync/database_document_test.go  | Modified           | JetBrains Junie (Google Gemini 3 Flash) |
| TESTS.md                                   | Written completely | JetBrains Junie (Google Gemini 3 Flash) |
| internal/kosync/models_test.go             | Written completely | JetBrains Junie (Google Gemini 3 Flash) |
| pkg/jmp/types_test.go                      | Written completely | JetBrains Junie (Google Gemini 3 Flash) |
| internal/kosync/api_auth_test.go           | Written completely | JetBrains Junie (Google Gemini 3 Flash) |
| pkg/jmp/jmp_test.go                        | Modified           | JetBrains Junie (Google Gemini 3 Flash) |
| internal/kosync/models.go                  | Modified           | JetBrains Junie (Google Gemini 3 Flash) |

## Commit Annotation
Since 2026-05-06 19:30 CEST all new commits where AI Agents or similar where used,  
have been annotated with the Git Trailers `AI-Agent` and `AI-Model`.

Some commits before that only mention the model or used `Co-Authored-By`.

The commit template `.gitmessage` has additional details about the format.
