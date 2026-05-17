# AI Usage

Sections of files and some files in whole where created by an AI Agent, LLM or similar.  
For transparency, below is a table with the files and how such an Agent interacted with it:

| File                                                    | Interaction        | Agemt (Model)                            |
|---------------------------------------------------------|--------------------|------------------------------------------|
| pkg/jmp-client-js                                       | Writen completely  | OpenCode (Ministral 3 - 3B Ollama)       |
| webui/src/api.ts                                        | Modified           | OpenCode (Ministral 3 - 3B Ollama)       |
| internal/kosync/migrations_test.go                      | Modified           | OpenCode (Ministral 3 - 3B Ollama)       |
| internal/kosync/crypto_test.go                          | Written partially  | OpenCode (Ministral 3 - 3B Ollama)       |
| internal/kosync/database_document_test.go               | Written partially  | OpenCode (Ministral 3 - 3B Ollama)       |
| internal/kosync/database_migration_test.go              | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/koreader_test.go                        | Written partially  | OpenCode (Ministral 3 - 3B Ollama)       |
| pkg/**/*_test.go                                        | Modified           | OpenCode (Ministral 3 - 3B Ollama)       |
| internal/kosync/config_test.go                          | Written completely | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/database_backup_test.go                 | Written completely | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/database_user_test.go                   | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/log_test.go                             | Written completely | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/models_test.go                          | Written completely | JetBrains Junie (Google Gemini 3 Flash)  |
| pkg/jmp/types_test.go                                   | Written completely | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/api_auth_test.go                        | Written completely | JetBrains Junie (Google Gemini 3 Flash)  |
| pkg/jmp/jmp_test.go                                     | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/models.go                               | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| webui/src/components/LoginModal.vue                     | Written completely | JetBrains Junie (Google Gemini 3 Flash)  |
| webui/src/stores/user.ts                                | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| webui/src/views/HomeView.vue                            | Modified           | JetBrains Junie (Google Gemini 3.1 Pro)  |
| webui/README.md                                         | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| webui/package.json                                      | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/database_document.go                    | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/api_syncs.go                            | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/websocket.go                            | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/api_socket_test.go                      | Written completely | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/api_webui.go                            | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/kosync.go                               | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/api_webui_test.go                       | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/api_syncs_test.go                       | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/api_users_test.go                       | Written completely | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/database.go                             | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/api_socket.go                           | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/database_backup.go                      | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/database_migration.go                   | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| pkg/decode/decode.go                                    | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/crypt_test.go                           | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| pkg/jmp-client-js/index.ts                              | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| pkg/jmp-client-js/README.md                             | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/config_test.go                          | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/database_document_test.go               | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/middleware.go                           | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| webui/src/components/DocumentsList.vue                  | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| webui/src/components/HistoryList.vue                    | Written completely | JetBrains Junie (Google Gemini 3 Flash)  |
| webui/src/main.ts                                       | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| webui/src/App.vue                                       | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| .junie/AGENTS.md                                        | Written partially  | JetBrains Junie (Google Gemini 3 Flash)  |
| CHANGELOG.md                                            | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| webui/vite.config.ts                                    | Modified           | JetBrains Junie (claude-sonnet-4-6)      |
| webui/src/tests/setup.ts                                | Written completely | JetBrains Junie (claude-sonnet-4-6)      |
| webui/src/tests/api.test.ts                             | Written completely | JetBrains Junie (claude-sonnet-4-6)      |
| webui/src/tests/stores/user.test.ts                     | Written completely | JetBrains Junie (claude-sonnet-4-6)      |
| webui/src/tests/stores/sync.test.ts                     | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| .gitlab-ci.yml                                          | Modified           | JetBrains Junie (claude-sonnet-4-6)      |
| internal/kosync/models_statistics.go                    | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/database_statistics_test.go             | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| webui/src/models/statistics.ts                          | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/migrations/sql/103-unify_timestamps.sql | Written completely | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/database_document_test.go               | Written completely | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/api_webui.go                            | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| internal/kosync/api_socket_test.go                      | Modified           | JetBrains Junie (Google Gemini 3 Flash)  |
| webui/src/components/TopBar.vue                         | Written completely | JetBrains Junie (gemini-3.1-pro-preview) |
| webui/src/components/DashboardMetrics.vue               | Written completely | JetBrains Junie (gemini-3.1-pro-preview) |
| CONTRIBUTING.md                                         | Written completely | JetBrains Junie (gemini-3.1-pro-preview) |
| pkg/jmp/jmp.go                                          | Modified           | JetBrains Junie (gemini-3.1-pro-preview) |
| pkg/jmp/types.go                                        | Modified           | JetBrains Junie (claude-sonnet-4-6) |
| internal/kosync/api_statistics.go                       | Modified           | JetBrains Junie (claude-sonnet-4-6) |
| webui/src/stores/sync.ts                                | Modified           | JetBrains Junie (claude-sonnet-4-6) |
| internal/kosync/database_statistics.go                  | Modified           | JetBrains Junie (claude-sonnet-4-6) |
| internal/kosync/api_socket.go                           | Modified           | JetBrains Junie (claude-sonnet-4-6) |
| webui/src/components/ReadStatisticsChart.vue            | Modified           | JetBrains Junie (claude-sonnet-4-6) |

The table is maintained on a best-effort basis and may not be exhaustive.
Each file is only listed once.

## Commit Annotation
Since 2026-05-06 19:30 CEST all new commits where AI Agents or similar where used,  
have been annotated with the Git Trailers `AI-Agent` and `AI-Model`.

Some commits before that only mention the model or used `Co-Authored-By`.

The commit template `.gitmessage` has additional details about the format.
