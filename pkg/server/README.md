# Unikorn Server

## Code Generation

Everything is done with an OpenAPI schema.
This allows us to auto-generate the server routing, schema validation middleware, types and clients.
This happens automatically on update via the `Makefile`.
Please ensure updated generated code is commited with your pull request.

## API Definition

Consult the [OpenAPI schema](../../pkg/server/openapi/server.spec.yaml) for full details of what it does.

## Getting Started with Development and Testing.

Once everything is up and running, grab the IP address:

```bash
export INGRESS_ADDR=$(kubectl -n unikorn get ingress/unikorn-server -o 'jsonpath={.status.loadBalancer.ingress[0].ip}')
```
And add it to your resolver:

```bash
echo "${INGRESS_ADDR} kubernetes.eschercloud.com" >> /etc/hosts
```

## API Testing

This does what a client is expected to do to bootstrap a session.
Copy this, make a script, whatever works for you!

### Get an Unscoped Token

When you first log in the the system you'll supply a username and password:

```bash
export TOKEN=$(curl -vkq https://kubernetes.eschercloud.com/api/v1/auth/tokens/password -X POST -u 'username:password' | jq  -r .token)
```

### Get a List of Projects

The token from the previous step allows very little functionality e.g. you cannot see any images or flavors.
We need a scoped token for that:

```bash
curl -vkq https://kubernetes.eschercloud.com/api/v1/providers/openstack/projects -H "Authorization: Bearer ${TOKEN}" | jq  .
```

It is anticipated that this step can be omitted most of the time for a Web application, as the project preference can be cached in `window.localStorage` to persist across sessions.

### Get a Scoped Token

Grab a token that is scoped to a specific project and you can begin hitting other APIs:

```bash
export TOKEN=$(curl -vkq https://kubernetes.eschercloud.com/api/v1/auth/tokens/token -H "Authorization: Bearer ${TOKEN}" -d '{"project":{"id":"23a9e437091d481da99f2aa07180b4ea"}}' | jq  -r token)
```
