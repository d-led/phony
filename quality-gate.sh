#!/usr/bin/env bash
#
# quality-gate.sh — how good is this repository today, and what to do next.
#
# The stages come in two kinds:
#
#   required   the repository meets it today, so a failure fails the gate.
#   advisory   a tighter check we have not adopted yet: it is reported, and it
#              shows up in "what to do next" at the end. It fails the gate only
#              with --strict.
#
# The gate gets tighter in two ways: a stage moves from advisory to required once
# it is clean, or a number in the configuration block below is lowered. Nothing
# here runs in CI yet, on purpose: the point of the script is to decide, from what
# it prints, what to adopt next.
#
# Usage:
#   ./quality-gate.sh                # the gate, installing what it is missing
#   ./quality-gate.sh --list         # the stages and their state
#   ./quality-gate.sh --only crap    # one stage, or a comma-separated list
#   ./quality-gate.sh --quick        # install nothing, run nothing slow
#   ./quality-gate.sh --deep         # the slow stages too: mutation, benchmarks, fuzzing
#   ./quality-gate.sh --strict       # advisory failures fail the gate as well
#
# Every stage writes its whole output to build/quality-gate/<stage>.log.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly ROOT
readonly OUT="$ROOT/build/quality-gate"
readonly TOOLDIR="$OUT/tools"
readonly EXAMPLE="$ROOT/examples/virtualtime-ticking"
readonly MODULES=("$ROOT" "$EXAMPLE")

# ---------------------------------------------------------------- how tight we are
# Tightening the gate is editing these numbers, or moving a stage from advisory
# to required where it is registered at the bottom.
readonly COVERAGE_MIN=85      # percent of statements covered, from `go tool cover`
readonly COMPLEXITY_MAX=8     # cyclomatic complexity of one function
readonly COGNITIVE_MAX=15     # cognitive complexity of one function
readonly CRAP_MAX=10          # per function: complexity^2 * (1-coverage)^3 + complexity
readonly DUPE_MAX_PERCENT=5   # percent of duplicated lines, jscpd
readonly DUPE_MIN_TOKENS=40   # what jscpd counts as a clone
readonly DUPE_MIN_LINES=4
readonly LINT_ADOPTED=errcheck,govet,staticcheck,unused,ineffassign,unparam,misspell,errorlint,bodyclose,nilerr,prealloc,goconst,revive

# ---------------------------------------------------------------- the tools we pin
readonly V_STATICCHECK=v0.8.2
readonly V_GOLANGCI=v2.14.0
readonly V_GOCYCLO=v0.6.0
readonly V_GOGNIT=v1.3.0
readonly V_GOFUMPT=v0.12.0
readonly V_ACTIONLINT=v1.7.12
readonly V_BENCHSTAT=v0.0.0-20261009192801-be2c69fb417e
readonly V_GREMLINS=v0.5.1
readonly V_JSCPD=4

# ---------------------------------------------------------------- the registry
# name, class, function: the order is the order of the run, and the class decides
# whether a failure fails the gate (required) or informs it (advisory).
readonly REGISTRY='
modules required stage_modules
fmt required stage_fmt
vet required stage_vet
build required stage_build
test required stage_test
shuffle required stage_shuffle
coverage required stage_coverage
staticcheck required stage_staticcheck
lint required stage_lint
workflows required stage_workflows
script required stage_script
complexity required stage_complexity
crap required stage_crap
gofumpt advisory stage_gofumpt
lint-strict advisory stage_lint_strict
security advisory stage_security
vulnerabilities advisory stage_vulnerabilities
memory advisory stage_memory
dupes advisory stage_dupes
mutation advisory stage_mutation
benchmarks advisory stage_benchmarks
fuzz advisory stage_fuzz
'

# ---------------------------------------------------------------- the running state
STRICT=false
QUICK=false
DEEP=false
LIST=false
ONLY=""
# One entry per stage that ran, in the order of the run. Plain arrays, because
# the bash that macOS ships is 3.2 and has no associative ones.
declare -a RAN=()
declare -a STATE=()
declare -a CLASS=()
declare -a NOTE=()

bold() { printf '\n\033[1m%s\033[0m\n' "$*"; }
label() {
	local path="${1#"$ROOT"}"
	path="${path#/}"
	printf '%s' "${path:-.}"
}

# tool echoes the path of a tool, installing the pinned version into
# build/quality-gate/tools if it is not on the PATH. It answers 1 when the tool
# cannot be had, which makes its stage skip.
tool() {
	local bin="$1" pkg="$2" version="$3"

	if command -v "$bin" >/dev/null 2>&1; then
		command -v "$bin"
		return 0
	fi

	if [[ -x "$TOOLDIR/$bin" ]]; then
		printf '%s\n' "$TOOLDIR/$bin"
		return 0
	fi

	if $QUICK; then
		return 1
	fi

	printf 'installing %s %s\n' "$bin" "$version" >&2
	if ! GOBIN="$TOOLDIR" go install "$pkg@$version" >&2; then
		return 1
	fi

	printf '%s\n' "$TOOLDIR/$bin"
}

skip() {
	printf 'SKIP: %s is not available here (%s)\n' "$1" "$2"
	return 77
}

# each_module runs a command in the library module and in the example module,
# which is a module of its own and replaces the library with the working copy.
each_module() {
	local dir failed=0
	for dir in "${MODULES[@]}"; do
		printf -- '--- %s ---\n' "$(label "$dir")"
		(cd "$dir" && "$@") || failed=1
	done
	return "$failed"
}

run_stage() {
	local name="$1" class="$2"
	shift 2

	if [[ -n "$ONLY" && ",$ONLY," != *",$name,"* ]]; then
		return
	fi

	mkdir -p "$OUT"
	local log="$OUT/$name.log"

	bold "$name ($class)"
	"$@" 2>&1 | tee "$log"
	local rc="${PIPESTATUS[0]}"

	local state
	case "$rc" in
	0) state=pass ;;
	77) state=skip ;;
	*) state=fail ;;
	esac

	RAN+=("$name")
	STATE+=("$state")
	CLASS+=("$class")
	NOTE+=("$(tail -n 1 "$log" | cut -c1-110)")
}

# ---------------------------------------------------------------- the toolchain
stage_modules() {
	local dir failed=0

	for dir in "${MODULES[@]}"; do
		printf -- '--- %s ---\n' "$(label "$dir")"
		(cd "$dir" && go mod verify && go mod tidy) || failed=1
	done

	if ! git -C "$ROOT" diff --quiet -- go.mod go.sum '*/go.mod' '*/go.sum'; then
		printf 'go mod tidy changed a module file:\n'
		git -C "$ROOT" --no-pager diff -- go.mod go.sum '*/go.mod' '*/go.sum'
		failed=1
	fi

	[[ "$failed" == 0 ]] && printf 'the module files are tidy and verified\n'
	return "$failed"
}

stage_fmt() {
	local unformatted
	unformatted="$(cd "$ROOT" && gofmt -l .)"

	if [[ -n "$unformatted" ]]; then
		printf 'gofmt wants to change:\n%s\n' "$unformatted"
		return 1
	fi

	printf 'gofmt: every file is formatted\n'
}

stage_vet() {
	each_module go vet ./... || return 1
	printf 'go vet: nothing to say\n'
}

stage_build() {
	# The example's binary goes into build/quality-gate/bin, so that building it
	# does not drop it into the working tree. The library has no main packages to
	# put anywhere, so it is built for the sake of building it.
	mkdir -p "$OUT/bin"
	(cd "$ROOT" && go build ./...) || return 1
	(cd "$EXAMPLE" && go build -o "$OUT/bin" ./...) || return 1

	printf 'go build: every package builds\n'
}

stage_test() {
	each_module go test -count=1 -race ./... || return 1
	printf 'go test -race: the suite passes\n'
}

# The same tests, in another order: a test that depends on the order of another
# one is a test that will fail in CI on a Tuesday.
stage_shuffle() {
	each_module go test -count=1 -shuffle=on ./... || return 1
	printf 'the suite passes in a shuffled order too\n'
}

stage_coverage() {
	if ! (cd "$ROOT" && go test -count=1 -covermode=atomic -coverprofile="$OUT/coverage.out" ./... >"$OUT/coverage-test.txt" 2>&1); then
		tail -20 "$OUT/coverage-test.txt"
		return 1
	fi

	(cd "$ROOT" && go tool cover -func="$OUT/coverage.out") >"$OUT/cover-func.txt"

	local total
	total="$(tail -n 1 "$OUT/cover-func.txt" | awk '{print $NF}' | tr -d '%')"

	printf 'coverage, per package:\n'
	grep 'coverage: ' "$OUT/coverage-test.txt"

	if awk -v total="$total" -v min="$COVERAGE_MIN" 'BEGIN { exit (total + 0 >= min) ? 0 : 1 }'; then
		printf 'coverage: %s%% of the statements, the gate wants %s%%\n' "$total" "$COVERAGE_MIN"
		return 0
	fi

	printf 'coverage: %s%% of the statements, below the gate of %s%%\n' "$total" "$COVERAGE_MIN"
	return 1
}

stage_staticcheck() {
	local bin
	bin="$(tool staticcheck honnef.co/go/tools/cmd/staticcheck "$V_STATICCHECK")" ||
		{ skip staticcheck "go install honnef.co/go/tools/cmd/staticcheck@$V_STATICCHECK"; return 77; }

	each_module "$bin" ./... || return 1
	printf 'staticcheck: nothing to say\n'
}

stage_lint() {
	local bin
	bin="$(tool golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint "$V_GOLANGCI")" ||
		{ skip golangci-lint "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$V_GOLANGCI"; return 77; }

	each_module "$bin" run --default=none --enable="$LINT_ADOPTED" || return 1
	printf 'golangci-lint: nothing to say about the adopted set\n'
}

stage_workflows() {
	local bin
	bin="$(tool actionlint github.com/rhysd/actionlint/cmd/actionlint "$V_ACTIONLINT")" ||
		{ skip actionlint "go install github.com/rhysd/actionlint/cmd/actionlint@$V_ACTIONLINT"; return 77; }

	"$bin" "$ROOT"/.github/workflows/*.yml || return 1
	printf 'actionlint: the workflows are sound\n'
}

stage_script() {
	command -v shellcheck >/dev/null 2>&1 || { skip shellcheck "brew install shellcheck"; return 77; }

	shellcheck "$ROOT"/quality-gate.sh || return 1
	printf 'shellcheck: this script is sound\n'
}

# ---------------------------------------------------------------- the tighter checks
# A survey, not a gate: golangci-lint with everything switched on, counted by rule,
# so that the next linter to adopt is the one with the most findings that matter.
stage_lint_strict() {
	local bin
	bin="$(tool golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint "$V_GOLANGCI")" ||
		{ skip golangci-lint "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$V_GOLANGCI"; return 77; }

	(cd "$ROOT" && "$bin" run --default=all --max-issues-per-linter=0 --max-same-issues=0) \
		>"$OUT/lint-strict-findings.txt" 2>&1 || true

	local findings
	findings="$(grep -cE '^[^ ]+\.go:[0-9]+:[0-9]+:' "$OUT/lint-strict-findings.txt")"
	printf '%s findings, by linter:\n' "$findings"
	grep -oE ' \([a-z0-9-]+\)$' "$OUT/lint-strict-findings.txt" | sort | uniq -c | sort -rn | head -12

	if [[ "$findings" == 0 ]]; then
		printf 'every linter golangci-lint has: nothing to say\n'
		return 0
	fi

	printf 'golangci-lint with everything on: %s findings, the heaviest rule is %s\n' \
		"$findings" "$(grep -oE ' \([a-z0-9-]+\)$' "$OUT/lint-strict-findings.txt" | sort | uniq -c | sort -rn | head -1 | awk '{print $1, $2}')"
	return 1
}

stage_complexity() {
	local cyclo cogni over_cyclo over_cogni
	cyclo="$(tool gocyclo github.com/fzipp/gocyclo/cmd/gocyclo "$V_GOCYCLO")" ||
		{ skip gocyclo "go install github.com/fzipp/gocyclo/cmd/gocyclo@$V_GOCYCLO"; return 77; }
	cogni="$(tool gocognit github.com/uudashr/gocognit/cmd/gocognit "$V_GOGNIT")" ||
		{ skip gocognit "go install github.com/uudashr/gocognit/cmd/gocognit@$V_GOGNIT"; return 77; }

	(cd "$ROOT" && "$cyclo" -over 0 .) >"$OUT/complexity.txt"
	(cd "$ROOT" && "$cogni" -over 0 .) >"$OUT/cognitive.txt"

	printf 'the most complex functions (cyclomatic, then cognitive):\n'
	sort -k1,1 -nr "$OUT/complexity.txt" | head -5
	sort -k1,1 -nr "$OUT/cognitive.txt" | head -5

	over_cyclo="$(awk -v max="$COMPLEXITY_MAX" '$1 > max' "$OUT/complexity.txt" | wc -l | tr -d ' ')"
	over_cogni="$(awk -v max="$COGNITIVE_MAX" '$1 > max' "$OUT/cognitive.txt" | wc -l | tr -d ' ')"
	printf 'over the gate: %s functions above complexity %s, %s above cognitive %s\n' \
		"$over_cyclo" "$COMPLEXITY_MAX" "$over_cogni" "$COGNITIVE_MAX"

	[[ "$over_cyclo" == 0 && "$over_cogni" == 0 ]]
}

# CRAP = complexity^2 * (1 - coverage)^3 + complexity, the change-risk metric: a
# complex function that no test covers is the one that will hurt. Coverage per
# function comes from go tool cover, complexity from gocyclo, and they are joined
# on the file and the name of the function.
stage_crap() {
	local cyclo
	cyclo="$(tool gocyclo github.com/fzipp/gocyclo/cmd/gocyclo "$V_GOCYCLO")" ||
		{ skip gocyclo "go install github.com/fzipp/gocyclo/cmd/gocyclo@$V_GOCYCLO"; return 77; }

	if [[ ! -s "$OUT/cover-func.txt" ]]; then
		if ! (cd "$ROOT" && go test -count=1 -covermode=atomic -coverprofile="$OUT/coverage.out" ./... >"$OUT/coverage.log" 2>&1); then
			tail -20 "$OUT/coverage.log"
			return 1
		fi
		(cd "$ROOT" && go tool cover -func="$OUT/coverage.out") >"$OUT/cover-func.txt"
	fi

	(cd "$ROOT" && "$cyclo" -over 0 .) >"$OUT/cyclo.txt"

	awk -v prefix="$(cd "$ROOT" && go list -m)/" '
		FILENAME == ARGV[1] {
			name = $3
			sub(/^\([^)]*\)\./, "", name)
			file = $4
			sub(/:[0-9]+:[0-9]+$/, "", file)
			sub(/^\.\//, "", file)
			if ($1 + 0 > complexity[file "|" name]) complexity[file "|" name] = $1 + 0
			next
		}
		{
			path = $1
			sub(/:$/, "", path)
			if (substr(path, 1, length(prefix)) == prefix) path = substr(path, length(prefix) + 1)

			complexity_of = complexity[path "|" $2]
			if (complexity_of == "") next

			covered = ($3 + 0) / 100
			crap = complexity_of * complexity_of * (1 - covered) ^ 3 + complexity_of
			printf "%.1f\t%d\t%.1f%%\t%s:%s\n", crap, complexity_of, covered * 100, path, $2
		}
	' "$OUT/cyclo.txt" "$OUT/cover-func.txt" | sort -k1,1 -gr >"$OUT/crap.txt"

	printf 'the riskiest functions (crap, complexity, coverage, function):\n'
	head -8 "$OUT/crap.txt"

	local threshold count
	for threshold in 30 20 15 10 8 5; do
		count="$(awk -v t="$threshold" '$1 > t' "$OUT/crap.txt" | wc -l | tr -d ' ')"
		printf '  %2s functions above a crap of %s\n' "$count" "$threshold"
	done

	count="$(awk -v t="$CRAP_MAX" '$1 > t' "$OUT/crap.txt" | wc -l | tr -d ' ')"
	if [[ "$count" != 0 ]]; then
		printf '%s functions are above the gate of %s:\n' "$count" "$CRAP_MAX"
		awk -v t="$CRAP_MAX" '$1 > t' "$OUT/crap.txt"
		return 1
	fi

	printf 'nothing is above the crap gate of %s\n' "$CRAP_MAX"
}

stage_gofumpt() {
	local bin
	bin="$(tool gofumpt mvdan.cc/gofumpt "$V_GOFUMPT")" ||
		{ skip gofumpt "go install mvdan.cc/gofumpt@$V_GOFUMPT"; return 77; }

	local unformatted
	unformatted="$(cd "$ROOT" && "$bin" -l .)"

	if [[ -n "$unformatted" ]]; then
		printf 'gofumpt, which is gofmt with more opinions, wants to change %s files:\n%s\n' \
			"$(printf '%s\n' "$unformatted" | wc -l | tr -d ' ')" "$unformatted"
		(cd "$ROOT" && "$bin" -d .) | head -40
		printf 'gofumpt wants %s files changed\n' "$(printf '%s\n' "$unformatted" | wc -l | tr -d ' ')"
		return 1
	fi

	printf 'gofumpt: nothing to change\n'
}

stage_security() {
	local bin failed=0
	bin="$(tool gosec github.com/securego/gosec/v2/cmd/gosec v2.29.0)" ||
		{ skip gosec "go install github.com/securego/gosec/v2/cmd/gosec@v2.29.0"; return 77; }

	each_module "$bin" -quiet ./... || failed=1

	if [[ "$failed" != 0 ]]; then
		printf 'gosec found something\n'
		return 1
	fi

	printf 'gosec: nothing to report\n'
}

stage_vulnerabilities() {
	local bin
	bin="$(tool govulncheck golang.org/x/vuln/cmd/govulncheck v1.9.0)" ||
		{ skip govulncheck "go install golang.org/x/vuln/cmd/govulncheck@v1.9.0"; return 77; }

	each_module "$bin" ./... || return 1
	printf 'govulncheck: nothing in the code this repository calls\n'
}

# The race detector is part of the test stage; this is the other half of the memory
# question, the memory and address sanitizers. The toolchain has them on linux/amd64
# and nowhere else, so elsewhere the stage says where it is skipping: a sanitizer
# that cannot run is not a sanitizer that passed.
stage_memory() {
	local platform sanitizer
	platform="$(go env GOOS)/$(go env GOARCH)"

	if [[ "$platform" != linux/amd64 ]]; then
		skip "the memory checkers" "the toolchain has -msan and -asan on linux/amd64 only, and this is $platform"
		return 77
	fi

	if [[ "$(go env CGO_ENABLED)" != 1 ]]; then
		skip "the memory checkers" "the sanitizers need cgo, which is off here"
		return 77
	fi

	if ! command -v clang >/dev/null 2>&1; then
		skip "the memory checkers" "the sanitizers are clang's, and clang is not here"
		return 77
	fi

	# The Go toolchain drives the sanitizers through the C compiler.
	export CC=clang

	for sanitizer in msan asan; do
		printf -- '--- go test -%s ---\n' "$sanitizer"
		each_module go test -count=1 "-$sanitizer" ./... || return 1
	done

	printf 'the memory sanitizer and the address sanitizer found nothing\n'
}

# The clone check the metric asks for: jscpd over the Go files of both modules.
stage_dupes() {
	local -a jscpd
	local found=0

	if command -v jscpd >/dev/null 2>&1; then
		jscpd=(jscpd)
	elif ! $QUICK && command -v npx >/dev/null 2>&1; then
		jscpd=(npx --yes "jscpd@$V_JSCPD")
	else
		skip jscpd "npm install -g jscpd"
		return 77
	fi

	(cd "$ROOT" && "${jscpd[@]}" \
		--min-tokens "$DUPE_MIN_TOKENS" --min-lines "$DUPE_MIN_LINES" \
		--format go --ignore '**/build/**,**/.git/**' \
		--reporters console,json --output "$OUT/jscpd" \
		--threshold "$DUPE_MAX_PERCENT" .) || found=1

	local report="$OUT/jscpd/jscpd-report.json"
	local percentage="?"
	if [[ -s "$report" ]]; then
		if command -v jq >/dev/null 2>&1; then
			percentage="$(jq -r '.statistics.total.percentage' "$report")"
		else
			percentage="$(grep -o '"percentage": *[0-9.]*' "$report" | tail -1 | tr -dc '0-9.')"
		fi
	fi

	printf 'jscpd: %s%% of the lines are duplicated (the gate wants at most %s%%)\n' \
		"$percentage" "$DUPE_MAX_PERCENT"

	if [[ "$found" != 0 ]]; then
		printf 'there are clones above the gate\n'
		return 1
	fi

	printf 'jscpd: within the gate\n'
}

# ---------------------------------------------------------------- the slow ones
# Mutation testing: change the code a little, see whether a test notices. The
# numbers are a survey and not a verdict: gremlins times each mutant's test run
# against a baseline, so on a busy machine a mutant times out instead of being
# killed, and the runs after the first one reuse what gremlins already knows.
# Run it on a quiet machine, and read the survivors rather than the percentage.
stage_mutation() {
	if ! $DEEP; then
		printf 'SKIP: mutation testing runs the tests once per mutant; use --deep\n'
		return 77
	fi

	local bin
	bin="$(tool gremlins github.com/go-gremlins/gremlins/cmd/gremlins "$V_GREMLINS")" ||
		{ skip gremlins "go install github.com/go-gremlins/gremlins/cmd/gremlins@$V_GREMLINS"; return 77; }

	# Two workers and a generous timeout: a mutant's test run pays for a rebuild of
	# the package on a machine that is already busy, and a mutant that times out is
	# not the same finding as a mutant that lives.
	(cd "$ROOT" && "$bin" unleash --threshold-efficacy 40 --workers 2 \
		--timeout-coefficient 6 --output "$OUT/mutation.json" ./virtualtime/) \
		>"$OUT/mutation-findings.txt" 2>&1
	local rc=$?

	tail -3 "$OUT/mutation-findings.txt"

	# A mutant that lives is a test that is not there, or a mutant that cannot be
	# killed: name the line it sits on either way.
	if grep -q LIVED "$OUT/mutation-findings.txt"; then
		printf 'mutants that lived, and the line they sit on:\n'
		local file line
		while read -r file line; do
			printf '  %s:%s  %s\n' "$file" "$line" \
				"$(sed -n "${line}p" "$ROOT/virtualtime/$file" | sed 's/^[[:space:]]*//')"
		done < <(grep LIVED "$OUT/mutation-findings.txt" | awk '{print $NF}' | tr ':' ' ' | awk '{print $1, $2}' | sort -u)
	fi

	if [[ "$rc" != 0 ]]; then
		printf 'mutation testing is below the threshold\n'
		return 1
	fi

	printf 'mutation: %s\n' "$(grep 'Test efficacy' "$OUT/mutation-findings.txt")"
}

stage_benchmarks() {
	if ! $DEEP; then
		printf 'SKIP: benchmarks take their time; use --deep\n'
		return 77
	fi

	(cd "$ROOT" && go test -run '^$' -bench . -benchmem -count 3 ./... >"$OUT/bench.txt") || return 1
	grep -E '^(Benchmark|ok|FAIL)' "$OUT/bench.txt"

	local bin
	if bin="$(tool benchstat golang.org/x/perf/cmd/benchstat "$V_BENCHSTAT")"; then
		if [[ -s "$OUT/bench-baseline.txt" ]]; then
			printf 'against the baseline of the last run that saved one:\n'
			"$bin" "$OUT/bench-baseline.txt" "$OUT/bench.txt" || return 1
		else
			cp "$OUT/bench.txt" "$OUT/bench-baseline.txt"
			printf 'no baseline yet: this run is now the baseline\n'
		fi
	fi
}

stage_fuzz() {
	if ! $DEEP; then
		printf 'SKIP: fuzzing runs for a minute; use --deep\n'
		return 77
	fi

	local targets
	targets="$(cd "$ROOT" && grep -rlE '^func Fuzz' --include='*_test.go' . || true)"

	if [[ -z "$targets" ]]; then
		printf 'there are no fuzz targets yet; the timer heap and the clock are the candidates\n'
		return 1
	fi

	local file target failed=0
	for file in $targets; do
		target="$(grep -oE '^func (Fuzz[A-Za-z0-9_]+)' "$ROOT/${file#./}" | head -1 | awk '{print $2}')"
		printf -- '--- %s ---\n' "$target"
		(cd "$ROOT" && go test -run '^$' -fuzz "^$target\$" -fuzztime 30s "./${file%/*}/") || failed=1
	done

	return "$failed"
}

# ---------------------------------------------------------------- the gate
usage() {
	cat <<'USAGE'
How good is this repository today, and what to do next.

Usage:
  ./quality-gate.sh                the gate, installing what it is missing
  ./quality-gate.sh --list         the stages and their class
  ./quality-gate.sh --only crap    one stage, or a comma-separated list
  ./quality-gate.sh --quick        install nothing, run nothing slow
  ./quality-gate.sh --deep         the slow stages too: mutation, benchmarks, fuzzing
  ./quality-gate.sh --strict       advisory failures fail the gate as well

Required stages fail the gate; advisory ones are reported and listed as things to
adopt. Every stage writes its output to build/quality-gate/<stage>.log.
USAGE
}

list_stages() {
	local name class fn

	printf 'stages of the gate:\n'
	while read -r name class fn; do
		[[ -z "$name" ]] && continue
		printf '  %-16s %s\n' "$name" "$class"
	done <<<"$REGISTRY"
}

main() {
	while [[ $# -gt 0 ]]; do
		case "$1" in
		--list) LIST=true ;;
		--quick) QUICK=true ;;
		--deep) DEEP=true ;;
		--strict) STRICT=true ;;
		--only)
			shift
			ONLY="${1:-}"
			;;
		--help | -h)
			usage
			return 0
			;;
		*)
			printf 'unknown argument: %s\n' "$1"
			return 2
			;;
		esac
		shift
	done

	if $LIST; then
		list_stages
		return 0
	fi

	mkdir -p "$OUT"

	local name class fn
	while read -r name class fn; do
		[[ -z "$name" ]] && continue
		run_stage "$name" "$class" "$fn"
	done <<<"$REGISTRY"

	summary
}

summary() {
	local i name state failed=0 advisories=0 clean=0 skipped=0

	bold 'the gate'
	for i in "${!RAN[@]}"; do
		printf '  %-8s %-16s %s\n' "${STATE[$i]}" "${RAN[$i]}" "${NOTE[$i]}"
	done

	bold 'what to do next'
	for i in "${!RAN[@]}"; do
		name="${RAN[$i]}"
		state="${STATE[$i]}"

		if [[ "${CLASS[$i]}" == required ]]; then
			if [[ "$state" == fail ]]; then
				failed=$((failed + 1))
			fi
			continue
		fi

		case "$state" in
		pass)
			clean=$((clean + 1))
			printf -- '- %s is clean: promote it to required in quality-gate.sh\n' "$name"
			;;
		skip)
			skipped=$((skipped + 1))
			printf -- '- %s did not run: %s\n' "$name" "${NOTE[$i]}"
			;;
		*)
			advisories=$((advisories + 1))
			printf -- '- %s: %s\n' "$name" "${NOTE[$i]}"
			if [[ $STRICT == true ]]; then
				failed=$((failed + 1))
			fi
			;;
		esac
	done

	cat <<'IDEAS'
- not adopted yet, roughly in the order worth trying:
  - nilaway (go.uber.org/nilaway/cmd/nilaway) for nil safety, and fieldalignment for
    struct padding: both are noisy, so they belong in a report first.
  - fuzz targets for the clock: a FuzzAdvance that fires random timers and checks
    the order and the instants of the trace.
  - benchstat baselines, so that a change in the benchmarks has to be explained.
  - go test -count=5 for the flakiness that one run hides, and -cpu=1,4 for the
    scheduling that one CPU hides.
  - golang.org/x/exp/apidiff between the last two tags, so that the library's API
    is only broken on purpose.
  - the same gate in CI (.github/workflows), once it has been green for a while.
IDEAS

	printf '\nadopted stages that failed: %s; advisory with findings: %s; advisory clean: %s; not run: %s\n' \
		"$failed" "$advisories" "$clean" "$skipped"

	if [[ "$failed" != 0 ]]; then
		printf 'the gate is red\n'
		return 1
	fi

	printf 'the gate is green\n'
}

main "$@"
