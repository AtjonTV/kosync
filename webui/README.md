# KOsync WebUI

This is the WebUI of KOsync, built using Vue 3, PrimeVue, Pinia and TailwindCSS.

The goal of the WebUI is to provide a simple and easy to use interface for managing KOsync.

## How it works

The WebUI requests special APIs made for it.

There are currently these endpoints:
- GET `/api/auth.jwt` for JWT login (requires `x-auth-user` and `x-auth-key` headers, where `x-auth-key` is the MD5 hash of the password).
- GET `/api/documents.all` which returns all documents in WebUI format.
- PUT `/api/documents.update` which allows updating the `pretty_name` field.

The API Route names are in a RPC function name format instead of traditional RESTful ones.

### Login Process

When a user wants to login and clicks the "Login" button, a modal opens asking for credentials.
The app then sends a request to `/api/auth.jwt` with the credentials in the `x-auth-user` (username) and `x-auth-key` (MD5 hash of password) headers.

The server responds with a JWT token, which the app stores in the `userStore`.

All subsequent API requests are made with the `Authorization: Bearer <token>` header.

Logout works by removing the user data from the `userStore`.

## Project Setup

```sh
bun install
```

### Compile and Hot-Reload for Development

Before you can develop the WebUI separate of KOsync, you must change the BASE_URL in `src/api.ts` to your local KOsync Address.  
You also have to make sure that the KOsync Server is running at that address.

This is required because in production the WebUI uses relative URIs for API calls.

With this precondition fulfilled, you can start the vite development server with:

```sh
bun dev
```

### Type-Check, Compile and Minify for Production

Production builds are handled by running `go generate kosync.go` in the project root directory.
