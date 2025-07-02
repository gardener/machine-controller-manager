#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Generate"

check_branch="__check"
stashed=false
checked_out=false
generated=false

function delete-check-branch {
  git rev-parse --verify "$check_branch" &>/dev/null && git branch -q -D "$check_branch" || :
}

function cleanup {
  if [[ "$generated" == true ]]; then
    if ! clean_err="$(make clean && git reset --hard -q && git clean -qdf)"; then
      echo "Could not clean: $clean_err"
    fi
  fi

  if [[ "$checked_out" == true ]]; then
    if ! checkout_err="$(git checkout -q -)"; then
      echo "Could not checkout to previous branch: $checkout_err"
    fi
  fi

  if [[ "$stashed" == true ]]; then
    if ! stash_err="$(git stash pop -q)"; then
      echo "Could not pop stash: $stash_err"
    fi
  fi

  delete-check-branch
}

trap cleanup EXIT SIGINT SIGTERM

if which git &>/dev/null; then
  git config --global user.name 'Gardener'
  git config --global user.email 'gardener@cloud'
  if ! git rev-parse --git-dir &>/dev/null; then
    echo "Not a git repo, aborting"
    exit 1
  fi

  if [[ "$(git rev-parse --abbrev-ref HEAD)" == "$check_branch" ]]; then
    echo "Already on check branch, aborting"
    exit 1
  fi
  delete-check-branch

  if [[ "$(git status -s)" != "" ]]; then
    stashed=true
    git stash --include-untracked -q
    git stash apply -q &>/dev/null
  fi

  checked_out=true
  git checkout -q -b "$check_branch"
  git add --all
  git commit -q --allow-empty -m 'checkpoint'

  old_status="$(git status -s)"

  if ! out=$(rm -rf hack/tools/bin/ 2>&1); then
    echo "Error while cleaning hack/tools/bin/: $out"
    exit 1
  fi

  echo ">> make generate"
  generated=true
  if ! out=$(make generate 2>&1); then
    echo "Error during calling make generate: $out"
    exit 1
  fi
  new_status="$(git status -s)"

  if [[ "$old_status" != "$new_status" ]]; then
    echo "make generate needs to be run:"
    echo "$new_status"
    exit 1
  fi

  echo ">> make tidy"
  if ! out=$(make tidy 2>&1); then
    echo "Error during calling make tidy: $out"
    exit 1
  fi
  new_status="$(git status -s)"

  if [[ "$old_status" != "$new_status" ]]; then
    echo "make tidy needs to be run:"
    echo "$new_status"
    exit 1
  fi
else
  echo "No git detected, cannot run make check-generate"
fi
exit 0