# Hivemind Local Instructions

Hivemind is a collaborative knowledge base for Discord guilds.

## Development Environment

- The default local workflow is Tilt. Kubernetes development uses a per-user namespace named `hivemind-<username>`.
- Use `make test` for the full Go test suite. Use `make proto` and `make assets` after changing protobufs or frontend assets.
- The local services expose server gRPC on `4153`, server metrics on `4163`, web HTTP on `8080`, and PostgreSQL on `5432`.

### Database Access

The Tilt Kubernetes workflow runs PostgreSQL in a Kubernetes pod, not a Docker container.

```bash
kubectl get pods -n hivemind-<username>
kubectl exec -it -n hivemind-<username> postgres-0 -- psql -U postgres -d hivemind
```

Do not use `docker exec hivemind-postgres`; that container does not exist in the Tilt/Kubernetes setup.

### PostgreSQL Intervals

When passing a Go `time.Duration` to a PostgreSQL interval parameter, convert it to seconds first. Passing the nanosecond value directly causes PostgreSQL range errors.
