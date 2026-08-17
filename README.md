# xorm v1 synchronization

This branch carries the GitHub Actions workflow that mirrors the `v1` branch
and all tags from `https://gitea.com/xorm/xorm.git` to this repository.

The workflow runs daily at 01:17 UTC (09:17 Asia/Shanghai) and can also be
started manually. It does not synchronize any other branch or delete target
tags that disappear from the source. A rewritten source tag is reported as a
conflict and is never forced over an existing target tag.
