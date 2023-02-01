= Unikorn Server

== Code Generation

Everything is done with an OpenAPI schema.
This allows us to auto-generate the server routing, schema validation middleware, types and clients.
This happens automatically on update via the `Makefile`.
Please ensure updated generated code is commited with your pull request.

== Deployment

Before you start ensure the following are installed:

* cert-manager (used to generate keying material for JWE/JWS and for ingress TLS)
* nginx-ingress (to perform routing, avoiding CORS, and TLS termination)

Server is part of the repository Helm chart, just add the following:

```
--set server.enabled=true
```

Once everything is up and running, grab the IP address:

```bash
export INGRESS_ADDR=$(kubectl -n unikorn get ingress/unikorn-server -o 'jsonpath={.status.loadBalancer.ingress[0].ip}')
```
And add it to your resolver:

```bash
echo "${INGRESS_ADDR} kubernetes.eschercloud.com" >> /etc/hosts
```

== API Testing

This does what a client is expected to do to bootstrap a session.
Copy this, make a script, whatever works for you!

=== Get an Unscoped Token

```bash
export TOKEN=$(curl -vkq https://kubernetes.eschercloud.com/api/v1/auth/tokens/password -X POST -u 'username:password' | jq  -r .token)
```

=== Get a List of Projects

It is anticipated that this step can be omitted most of the time for a Web application, as the project preference can be cached in `window.localStorage` to persist across sessions.

```bash
curl -vkq https://kubernetes.eschercloud.com/api/v1/providers/openstack/projects -H "Authorization: Bearer ${TOKEN}" | jq  .
```
