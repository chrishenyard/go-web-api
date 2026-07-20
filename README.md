# go-web-api

A Golang web API demonstrating authentication at the **class level** (struct/handler group) and **function level** (individual endpoints).

---

## Authentication architecture

### Class-level authentication
All routes registered under a handler struct (e.g. `UserHandler`, `AdminHandler`) are wrapped with `middleware.RequireAuth`. This middleware validates a signed JWT on every request before the handler code is ever reached. If the token is missing or invalid the request is rejected with **401 Unauthorized**.

```
POST /api/auth/register  ← public
POST /api/auth/login     ← public
GET  /api/users/profile  ← class-level JWT gate (UserHandler)
GET  /api/users          ← class-level JWT gate (UserHandler)
DELETE /api/users/{id}   ← class-level JWT gate + function-level role gate
GET  /api/admin/stats    ← class-level JWT gate (AdminHandler) + function-level role gate
POST /api/admin/promote  ← class-level JWT gate (AdminHandler) + function-level role gate
```

### Function-level authentication
Inside individual handler methods, `middleware.RequireRole` performs a role check against the JWT claims already stored in the request context by `RequireAuth`. A **403 Forbidden** is returned when the caller's role does not satisfy the requirement.

For example, `UserHandler.Delete` and every method on `AdminHandler` are only accessible to users with the `admin` role, even though a plain `user` JWT would pass the class-level gate.

---

## Quick start

```bash
go run .        # starts server on :8080
```

### Register
```bash
curl -s -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","email":"alice@example.com","password":"s3cret"}'
# {"token":"<jwt>"}
```

### Login
```bash
curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"s3cret"}'
# {"token":"<jwt>"}
```

### Authenticated request (class-level gate)
```bash
TOKEN=<jwt from login>
curl http://localhost:8080/api/users/profile \
  -H "Authorization: Bearer $TOKEN"
```

### Admin-only request (class + function-level gate)
```bash
TOKEN=<admin jwt>
curl http://localhost:8080/api/admin/stats \
  -H "Authorization: Bearer $TOKEN"
```

---

## Running tests

```bash
go test ./...
```

### Kubernetes
## Add Secret
- vault kv put kv-v2/go-web-api/config \
  jwt_secret=""

## Get Secret
- vault kv get kv-v2/go-web-api/config

## Add App Policy
- vault policy write go-web-api-policy - <<EOF
path "kv-v2/*" {
capabilities = ["create", "read", "update", "list", "delete"]
}
EOF

## Add App Role
- vault write auth/approle/role/go-web-api-role \
  token_policies=default,go-web-api-policy \
  token_ttl=30d

## Read App Role
- vault read auth/approle/role/go-web-api-role

## Add Kubernetes Role
- vault write auth/kubernetes/role/go-web-api-role \
  bound_service_account_names=default \
  bound_service_account_namespaces=go-web-api \
  policies=default,go-web-api-policy \
  audience="https://kubernetes.default.svc.cluster.local" \
  ttl=30d

## Read Kubernetes Policy
- vault read auth/kubernetes/role/go-web-api-role

## Get App Role ID and Secret ID
- vault read auth/approle/role/go-web-api-role/role-id
- vault write -f auth/approle/role/go-web-api-role/secret-id

## Execute write operation to get auth token
- vault write auth/approle/login role_id="" secret_id=""
