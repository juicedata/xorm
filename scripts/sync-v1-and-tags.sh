#!/usr/bin/env bash

set -euo pipefail

readonly source_repository="${SOURCE_REPOSITORY:-https://gitea.com/xorm/xorm.git}"
readonly source_branch_ref="refs/heads/v1"
readonly source_branch_tracking_ref="refs/remotes/gitea/v1"
readonly source_tag_namespace="refs/gitea-tags"
readonly target_branch_ref="refs/heads/v1"

source_tags_file="$(mktemp)"
target_tags_file="$(mktemp)"
trap 'rm -f "${source_tags_file}" "${target_tags_file}"' EXIT

git for-each-ref --format='delete %(refname)' "${source_tag_namespace}/" |
  git update-ref --stdin

git fetch --force --no-tags \
  "${source_repository}" \
  "+${source_branch_ref}:${source_branch_tracking_ref}" \
  "+refs/tags/*:${source_tag_namespace}/*"

source_branch_sha="$(git rev-parse --verify "${source_branch_tracking_ref}^{commit}")"
target_branch_sha="$(git ls-remote --heads origin "${target_branch_ref}" | awk 'NR == 1 {print $1}')"

git for-each-ref \
  --format='%(objectname)%09%(refname)' \
  "${source_tag_namespace}/" > "${source_tags_file}"

refspecs=("${source_branch_tracking_ref}:${target_branch_ref}")
tag_count=0

while IFS=$'\t' read -r _ source_tag_ref; do
  [[ -n "${source_tag_ref}" ]] || continue

  tag_name="${source_tag_ref#${source_tag_namespace}/}"
  refspecs+=("${source_tag_ref}:refs/tags/${tag_name}")
  tag_count=$((tag_count + 1))
done < "${source_tags_file}"

if ! git push \
  --atomic \
  "--force-with-lease=${target_branch_ref}:${target_branch_sha}" \
  origin \
  "${refspecs[@]}"; then
  echo "atomic synchronization failed; no branch or tag update was accepted" >&2
  exit 1
fi

verified_branch_sha="$(git ls-remote --exit-code --heads origin "${target_branch_ref}" | awk 'NR == 1 {print $1}')"
if [[ "${verified_branch_sha}" != "${source_branch_sha}" ]]; then
  echo "v1 verification failed: source=${source_branch_sha} target=${verified_branch_sha}" >&2
  exit 1
fi

git ls-remote --tags origin 'refs/tags/*' |
  awk '$2 !~ /\^\{\}$/ {print $1 "\t" $2}' > "${target_tags_file}"

while IFS=$'\t' read -r source_tag_sha source_tag_ref; do
  [[ -n "${source_tag_ref}" ]] || continue

  tag_name="${source_tag_ref#${source_tag_namespace}/}"
  target_tag_ref="refs/tags/${tag_name}"
  verified_tag_sha="$(awk -F '\t' -v ref="${target_tag_ref}" '$2 == ref {print $1; exit}' "${target_tags_file}")"
  if [[ "${verified_tag_sha}" != "${source_tag_sha}" ]]; then
    echo "tag verification failed: ${tag_name} source=${source_tag_sha} target=${verified_tag_sha}" >&2
    exit 1
  fi
done < "${source_tags_file}"

echo "v1 synchronization verified at ${verified_branch_sha}"
echo "tag synchronization verified for ${tag_count} source tag(s)"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "### v1 and tag synchronization"
    echo
    echo "- Source: \`${source_repository}\`"
    echo "- v1: verified at \`${verified_branch_sha}\`"
    echo "- Source tags verified: ${tag_count}"
    echo "- Tag deletion: disabled"
  } >> "${GITHUB_STEP_SUMMARY}"
fi
