# KOsync WebUI

This is the WebUI of KOsync, built using Vue 3, PrimeVue, Pinia and TailwindCSS.

The goal of the WebUI is to provide a simple and easy to use interface for managing KOsync.

## How it works

The WebUI requests special APIs made for it.

There are currently three endpoints:
- GET `/api/auth.basic` for HTTP-Basic-Auth login.
- GET `/api/documents.all` which returns all documents in WebUI format.
- PUT `/api/documents.update` which allows updating the `pretty_name` field.

The API Route names are in a RPC function name format instead of traditional RESTful ones.

### Login Process

When a user wants to login and clicks the "Login" button, the app redirects the user to `/api/auth.basic`.  
This endpoint will ask the browser to perform HTTP-Basic-Auth.

After the HTTP-Basic-Auth the server will send a redirect to `/web?user=<base64_encoded_userdata>`.  
The app then reads the encoded data from the URL, decodes it and stores it in the `userStore`.

In the user data the username and password-hash are included.  
Because calculating MD5 hashes in JavaScript is not possible without legacy libraries, it gets the hash from the server.

The reason why the app received the password hash at all, is because that is the authentication mechanism used by  
KOReader's Sync Plugin, and the fact that I did not want to build a second mechanism.

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
