#!/usr/bin/env bash
set -euo pipefail

# Run the InsertIgnore integration tests against one isolated database image.
image=${1:?usage: test-insert-ignore.sh mysql:5.7|mariadb:10.11|postgres:9.5}
name="xorm-insert-ignore-$$"
docker_args=(run -d --name "$name")
if [[ -n ${XORM_DOCKER_PLATFORM:-} ]]; then
    docker_args+=(--platform "$XORM_DOCKER_PLATFORM")
fi
case "$image" in
    mysql:*|mariadb:*)
        driver=mysql
        port=3306
        envs=(-e MYSQL_ALLOW_EMPTY_PASSWORD=yes -e MYSQL_ROOT_HOST=%)
        if [[ $image == mariadb:* ]]; then
            ready=(mariadb-admin --protocol=tcp -h127.0.0.1 -uroot ping)
        else
            ready=(mysqladmin --protocol=tcp -h127.0.0.1 -uroot ping)
        fi
        ;;
    postgres:*)
        driver=postgres
        port=5432
        envs=(-e POSTGRES_PASSWORD=xorm)
        ready=(pg_isready -h127.0.0.1 -U postgres)
        ;;
    *) echo "Unsupported image: $image" >&2; exit 2 ;;
esac
cleanup() {
    docker logs "$name" > "${TMPDIR:-/tmp}/${name}.log" 2>&1 || true
    docker rm -f -v "$name" >/dev/null 2>&1 || true
}
trap 'cleanup_status=$?; cleanup; exit "$cleanup_status"' EXIT
# An automatically assigned loopback port avoids existing local databases.
docker "${docker_args[@]}" "${envs[@]}" -p "127.0.0.1::$port" "$image" >/dev/null
host_port=$(docker port "$name" "$port/tcp")
for ((i=0; i<120; i++)); do
    if docker exec "$name" "${ready[@]}" >/dev/null 2>&1; then
        break
    fi
    if ((i == 119)); then
        docker logs "$name" >&2
        exit 1
    fi
    sleep 1
done
docker image inspect "$image" --format '{{json .RepoDigests}}'
if [[ $driver == mysql ]]; then
    dsn="root:@tcp($host_port)/xorm_test?charset=utf8mb4&parseTime=true"
else
    dsn="postgres://postgres:xorm@$host_port/xorm_test?sslmode=disable"
fi
go test ./tests -run '^TestInsertIgnore' -count=1 -v -db="$driver" -conn_str="$dsn" -show_sql=false
if [[ $driver == postgres ]]; then
    go test ./tests -run '^TestInsertIgnore' -count=1 -v -db=pgx -conn_str="$dsn" -show_sql=false
fi
