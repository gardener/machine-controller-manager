#!/usr/bin/env bash

# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0
#
# Runs the controller IT suite repeatedly.

set -o errexit
set -o nounset
set -o pipefail

# ----- defaults -----
RUNS=10
BASE_DIR="hack/logs"
OUT_DIR="${BASE_DIR}/$(date +%Y%m%d-%H%M%S)"
STOP_ON_FAIL=false
VERBOSE=false
FOCUS=""

function usage() {
  cat <<EOF
Usage: $(basename "$0") [-n RUNS] [-o OUT_DIR] [-f FOCUS] [-s] [-v]

  -n RUNS      Number of times to run the integration test suite (default: ${RUNS})
  -o OUT_DIR   Directory to write per-run logs and the summary (default: ${BASE_DIR}/<timestamp>)
  -f FOCUS     Run only specs matching FOCUS (passed to ginkgo as --focus)
  -s           Stop early on the first failing run (default: run all)
  -v           Verbose: stream each run's output to the terminal
  -h           Show this help
EOF
}

function parse_flags() {
  while getopts ":n:o:f:svh" opt; do
    case "${opt}" in
      n) RUNS="${OPTARG}" ;;
      o) OUT_DIR="${OPTARG}" ;;
      f) FOCUS="${OPTARG}" ;;
      s) STOP_ON_FAIL=true ;;
      v) VERBOSE=true ;;
      h) usage; exit 0 ;;
      :) echo "Error: -${OPTARG} requires an argument." >&2; usage; exit 2 ;;
      \?) echo "Error: unknown option -${OPTARG}." >&2; usage; exit 2 ;;
    esac
  done

  # ----- validation -----
  if ! [[ "${RUNS}" =~ ^[0-9]+$ ]] || [[ "${RUNS}" -lt 1 ]]; then
    echo "Error: RUNS (-n) must be a positive integer, got '${RUNS}'." >&2
    exit 2
  fi
}

function run_suite() {
  local start_run i log_file run_start elapsed status label
  local -a focus_args=()
  [[ -n "${FOCUS}" ]] && focus_args=(--focus "${FOCUS}")

  local -a ginkgo_cmd=(ginkgo -v --no-color --show-node-events --poll-progress-after=300s --poll-progress-interval=60s "${focus_args[@]}")

  local pass=0 fail=0 total_elapsed=0
  local -a failed_runs=() pass_durations=() fail_durations=()

  start_run=$(date +%s)
  for i in $(seq 1 "${RUNS}"); do
    log_file="${OUT_DIR}/run_${i}.log"
    run_start=$(date +%s)
    status=0

    if [[ "${VERBOSE}" == true ]]; then
      printf "Run %s/%s (log: %s)\n" "${i}" "${RUNS}" "${log_file}"
      label="Run ${i}/${RUNS} "
      (cd test/integration/controller; "${ginkgo_cmd[@]}") 2>&1 | tee "${log_file}" || status=$?
    else
      printf "Run %s/%s ... " "${i}" "${RUNS}"
      label=""
      (cd test/integration/controller; "${ginkgo_cmd[@]}") >"${log_file}" 2>&1 || status=$?
    fi

    elapsed=$(( $(date +%s) - run_start ))

    if [[ "${status}" -eq 0 ]]; then
      pass=$(( pass + 1 ))
      pass_durations+=("${elapsed}")
      printf "%sPASS (%ds)\n" "${label}" "${elapsed}"
    else
      fail=$(( fail + 1 ))
      failed_runs+=("${i}")
      fail_durations+=("${elapsed}")
      printf "%sFAIL (exit %d, %ds) -> %s\n" "${label}" "${status}" "${elapsed}" "${log_file}"
      [[ "${STOP_ON_FAIL}" == true ]] && { echo "Stopping early on first failure (-s)."; break; }
    fi
  done
  total_elapsed=$(( $(date +%s) - start_run ))

  generate_report
}

function fmt_duration() {
  local s="$1"
  if [[ "${s}" -ge 60 ]]; then
    printf '%dm %02ds' "$(( s / 60 ))" "$(( s % 60 ))"
  else
    printf '%ds' "${s}"
  fi
}

function stats_of() {
  local -a arr=("$@")
  STAT_MIN=""; STAT_MAX=""; STAT_AVG=""
  local n="${#arr[@]}"
  [[ "${n}" -eq 0 ]] && return
  local sum=0 v
  STAT_MIN="${arr[0]}"; STAT_MAX="${arr[0]}"
  for v in "${arr[@]}"; do
    (( v < STAT_MIN )) && STAT_MIN="${v}"
    (( v > STAT_MAX )) && STAT_MAX="${v}"
    sum=$(( sum + v ))
  done
  STAT_AVG=$(( (sum + n / 2) / n ))   # rounded average
}

function generate_report() {
  local total_runs=$(( pass + fail ))
  {
    echo "=========================================="
    echo " Test summary"
    echo "=========================================="
    echo "Date         : $(date)"
    echo "Requested    : ${RUNS} run(s)"
    echo "Executed     : ${total_runs} run(s)"
    echo "Passed       : ${pass}"
    echo "Failed       : ${fail}"
    if [[ "${total_runs}" -gt 0 ]]; then
      printf "Pass rate    : %d%% (%d/%d)\n" "$(( pass * 100 / total_runs ))" "${pass}" "${total_runs}"
    fi
    echo "Wall-clock   : $(fmt_duration "${total_elapsed}") (includes inter-run overhead)"
    echo
    if [[ "${pass}" -gt 0 ]]; then
      stats_of "${pass_durations[@]}"
      echo "Runtime per run (passing only):"
      printf "  best       : %s\n" "$(fmt_duration "${STAT_MIN}")"
      printf "  worst      : %s\n" "$(fmt_duration "${STAT_MAX}")"
      printf "  average    : %s\n" "$(fmt_duration "${STAT_AVG}")"
    else
      echo "  (no runs succeeded)"
    fi
    echo "Logs dir     : ${OUT_DIR}"
    if [[ "${fail}" -gt 0 ]]; then
      echo "Failed runs  :"
      for idx in "${!failed_runs[@]}"; do
        printf "  - run_%s.log (%s)\n" "${failed_runs[$idx]}" "$(fmt_duration "${fail_durations[$idx]}")"
      done
    fi
    echo "=========================================="
  } | tee "${SUMMARY_FILE}"

  [[ "${fail}" -eq 0 ]]
}

function main() {
  parse_flags "$@"

  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  cd "${script_dir}" || { echo "Error: cannot cd to ${script_dir}" >&2; exit 1; }

  SUMMARY_FILE="${OUT_DIR}/summary.log"
  mkdir -p "${OUT_DIR}"

  echo "Running test-integration ${RUNS} time(s); logs in ${OUT_DIR}"
  echo

  run_suite
}

main "$@"
