# Drexa Backend — API Reference

Base URL: `http://localhost:8080/api/v1`

---

## Authentication

Protected endpoints require a valid JWT access token. Send it in one of two ways:

| Method | Value |
|--------|-------|
| Header | `Authorization: Bearer <access_token>` |
| Cookie | `access_token=<access_token>` |

Tokens are issued at login and refreshed via `/auth/refresh`. The server also sets `HttpOnly` cookies automatically on login/refresh responses.

---

## Common Responses

All responses are JSON.

```json
{ "message": "..." }   // success
{ "error": "..." }     // failure
```

| Status | Meaning |
|--------|---------|
| `200` | OK |
| `201` | Created |
| `400` | Bad request / validation error |
| `401` | Unauthorized / invalid credentials |
| `409` | Conflict (e.g. email already exists) |
| `500` | Internal server error |

---

## Auth

### Register

```
POST /auth/register
```

Creates a new account and sends an OTP to the provided email.

**Request**
```json
{
  "email": "user@example.com",
  "password": "securepassword"
}
```

**Response `201`**
```json
{ "message": "registration successful, check your email for the OTP" }
```

---

### Login

```
POST /auth/login
```

Authenticates with email and password. Sets `access_token` and `refresh_token` cookies on success.

**Request**
```json
{
  "email": "user@example.com",
  "password": "securepassword"
}
```

**Response `200`**
```json
{ "message": "login successful" }
```

---

### Logout

```
POST /auth/logout
```

Revokes the current refresh token from the session.

**Request** — no body required (reads `refresh_token` cookie)

**Response `200`**
```json
{ "message": "logged out" }
```

---

### Logout All Devices

```
POST /auth/logout/all
```

🔒 **Protected**

Revokes all active sessions for the authenticated user.

**Response `200`**
```json
{ "message": "all sessions revoked" }
```

---

### Refresh Token

```
POST /auth/refresh
```

Issues a new access token and rotates the refresh token. Reads `refresh_token` cookie.

**Response `200`**
```json
{ "message": "token refreshed" }
```

---

## Email / Phone Verification

### Verify Email

```
POST /auth/verify/email
```

Verifies the OTP sent to the user's email after registration.

**Request**
```json
{
  "user_id": "uuid",
  "otp": "123456"
}
```

**Response `200`**
```json
{ "message": "email verified" }
```

---

### Verify Phone

```
POST /auth/verify/phone
```

Verifies the OTP sent to the user's phone number.

**Request**
```json
{
  "user_id": "uuid",
  "otp": "123456"
}
```

**Response `200`**
```json
{ "message": "phone verified" }
```

---

## Password

### Request Password Reset

```
POST /auth/password/reset
```

Sends a password reset link to the email if it exists. Always returns `200` to prevent email enumeration.

**Request**
```json
{
  "email": "user@example.com"
}
```

**Response `200`**
```json
{ "message": "if that email exists you will receive a reset link" }
```

---

### Confirm Password Reset

```
POST /auth/password/reset/confirm
```

Resets the password using the token from the reset email. Revokes all active sessions on success.

**Request**
```json
{
  "token": "raw_reset_token_from_email",
  "new_password": "newsecurepassword"
}
```

**Response `200`**
```json
{ "message": "password reset successful" }
```

---

### Change Password

```
POST /auth/password/change
```

🔒 **Protected**

Changes password for the authenticated user. Requires the current password.

**Request**
```json
{
  "old_password": "currentpassword",
  "new_password": "newsecurepassword"
}
```

**Response `200`**
```json
{ "message": "password changed" }
```

---

## Trading PIN

### Set PIN

```
POST /auth/pin/set
```

🔒 **Protected**

Sets or updates the trading PIN. Required before executing trades or withdrawals.

**Request**
```json
{
  "pin": "123456"
}
```

**Response `200`**
```json
{ "message": "trading pin set" }
```

---

### Verify PIN

```
POST /auth/pin/verify
```

🔒 **Protected**

Verifies the trading PIN before a sensitive action.

**Request**
```json
{
  "pin": "123456"
}
```

**Response `200`**
```json
{ "message": "pin verified" }
```

---

## OAuth (Firebase)

OAuth endpoints require a Firebase ID token obtained from the frontend Firebase SDK after the user signs in with Google, Apple, etc.

### OAuth Register

```
POST /auth/oauth/register
```

Registers a new account using a verified Firebase identity.

**Request**
```json
{
  "id_token": "firebase_id_token_from_frontend"
}
```

**Response `201`**
```json
{ "message": "oauth registration successful" }
```

---

### OAuth Login

```
POST /auth/oauth/login
```

Logs in an existing account via Firebase identity. Sets `access_token` and `refresh_token` cookies on success.

**Request**
```json
{
  "id_token": "firebase_id_token_from_frontend"
}
```

**Response `200`**
```json
{ "message": "oauth login successful" }
```
