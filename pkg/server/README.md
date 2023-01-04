= Unikorn Server

== Code Generation

Everything is done with an OpenAPI schema.
This allows us to auto-generate the server routing, schema validation middleware, types and clients.

To generate the server routing:

```shell
go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.12.4
oapi-codegen -generate spec -package spec openapi/server.spec.yaml > schema/schema.go
oapi-codegen -generate types -package router openapi/server.spec.yaml > router/types.go
oapi-codegen -generate chi-server -package router openapi/server.spec.yaml > router/router.go
```
