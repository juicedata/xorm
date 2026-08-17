#!/usr/bin/env bash

set -euo pipefail

readonly source_repository="${SOURCE_REPOSITORY:-https://gitea.com/xorm/xorm.git}"
readonly source_ref="refs/heads/v1"
readonly source_tracking_ref="refs/remotes/gitea/v1"
readonly target_ref="refs/heads/v1"

git fetch --force --no-tags \
  "${source_repository}" \
  "+${source_ref}:${source_tracking_ref}"

source_sha="$(git rev-parse --verify "${source_tracking_ref}^{commit}")"
target_sha="$(git ls-remote --heads origin "${target_ref}" | awk 'NR == 1 {print $1}')"

if [[ "${source_sha}" == "${target_sha}" ]]; then
  result="No change"
else
  git push \
    "--force-with-lease=${target_ref}:${target_sha}" \
    origin \
    "${source_tracking_ref}:${target_ref}"
  result="Updated"
fi

verified_sha="$(git ls-remote --exit-code --heads origin "${target_ref}" | awk 'NR == 1 {print $1}')"
if [[ "${verified_sha}" != "${source_sha}" ]]; then
  echo "v1 verification failed: source=${source_sha} target=${verified_sha}" >&2
  exit 1
fi

echo "v1 synchronization: ${result} (${source_sha})"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "### v1 synchronization"
    echo
    echo "- Result: ${result}"
    echo "- Source: \`${source_repository}#v1\`"
    echo "- Previous target SHA: \`${target_sha:-missing}\`"
    echo "- Verified target SHA: \`${verified_sha}\`"
  } >> "${GITHUB_STEP_SUMMARY}"
fi
