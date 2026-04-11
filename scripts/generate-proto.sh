#!/usr/bin/env bash
# generate-proto.sh — Compile .proto definitions for Go and Android (Kotlin lite)
#
# Usage:
#   ./scripts/generate-proto.sh          # compile all targets
#   ./scripts/generate-proto.sh --go     # Go only
#   ./scripts/generate-proto.sh --kotlin # Kotlin only
#
# Dependencies:
#   Go output:    protoc, protoc-gen-go
#   Kotlin output: protoc, protoc-gen-kotlin (via protoc-kotlin artifact)

set -euo pipefail

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROTO_DIR="${PROJECT_ROOT}/protocol/proto"
GO_OUT="${PROJECT_ROOT}/backend/pkg/protocol"
KOTLIN_OUT="${PROJECT_ROOT}/android/app/src/main/proto/generated"

# ---------------------------------------------------------------------------
# Colour helpers
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Colour

info()    { echo -e "${BLUE}[proto]${NC} $*"; }
success() { echo -e "${GREEN}[proto]${NC} $*"; }
warn()    { echo -e "${YELLOW}[proto]${NC} $*"; }
error()   { echo -e "${RED}[proto] ERROR:${NC} $*" >&2; }

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
DO_GO=true
DO_KOTLIN=true

for arg in "$@"; do
  case "${arg}" in
    --go)     DO_KOTLIN=false ;;
    --kotlin) DO_GO=false ;;
    --help|-h)
      echo "Usage: $0 [--go] [--kotlin]"
      echo ""
      echo "Compile Protocol Buffer definitions to Go and/or Kotlin (lite)."
      echo ""
      echo "Options:"
      echo "  --go       Compile for Go only"
      echo "  --kotlin   Compile for Kotlin only (placeholder — Android setup required)"
      echo "  --help     Show this message"
      exit 0
      ;;
    *)
      error "Unknown argument: ${arg}"
      exit 1
      ;;
  esac
done

# ---------------------------------------------------------------------------
# Dependency checks
# ---------------------------------------------------------------------------
check_binary() {
  local bin="$1"
  local install_hint="$2"
  if ! command -v "${bin}" &>/dev/null; then
    error "'${bin}' not found."
    error "Install hint: ${install_hint}"
    return 1
  fi
  info "Found: $(command -v "${bin}") ($(${bin} --version 2>&1 | head -1))"
  return 0
}

MISSING=0

info "Checking dependencies..."

# protoc is always required
if ! check_binary protoc "brew install protobuf  |  apt-get install protobuf-compiler  |  https://github.com/protocolbuffers/protobuf/releases"; then
  MISSING=1
fi

if ${DO_GO}; then
  if ! check_binary protoc-gen-go "go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; then
    MISSING=1
  fi
fi

if ${DO_KOTLIN}; then
  # protoc-gen-kotlin is distributed as a JAR executed via a wrapper script.
  # The standard wrapper is 'protoc-gen-kotlin'; if absent we warn and skip.
  if ! command -v protoc-gen-kotlin &>/dev/null; then
    warn "'protoc-gen-kotlin' not found — skipping Kotlin output."
    warn "To enable: download protoc-gen-kotlin from https://github.com/open-telemetry/opentelemetry-proto"
    warn "           or use the Android Gradle protobuf plugin (recommended for Android projects)."
    DO_KOTLIN=false
  else
    info "Found: $(command -v protoc-gen-kotlin)"
  fi
fi

if [[ ${MISSING} -eq 1 ]]; then
  error "One or more required dependencies are missing. Aborting."
  exit 1
fi

# ---------------------------------------------------------------------------
# Collect .proto files
# ---------------------------------------------------------------------------
PROTO_FILES=()
while IFS= read -r -d '' f; do
  PROTO_FILES+=("${f}")
done < <(find "${PROTO_DIR}" -name "*.proto" -print0 | sort -z)

if [[ ${#PROTO_FILES[@]} -eq 0 ]]; then
  error "No .proto files found in ${PROTO_DIR}"
  exit 1
fi

info "Found ${#PROTO_FILES[@]} proto file(s):"
for f in "${PROTO_FILES[@]}"; do
  info "  ${f#"${PROJECT_ROOT}/"}"
done

# ---------------------------------------------------------------------------
# Go compilation
# ---------------------------------------------------------------------------
compile_go() {
  info "Compiling Go output → ${GO_OUT#"${PROJECT_ROOT}/"}"
  mkdir -p "${GO_OUT}"

  protoc \
    --proto_path="${PROTO_DIR}" \
    --go_out="${GO_OUT}" \
    --go_opt=paths=source_relative \
    "${PROTO_FILES[@]}"

  success "Go compilation complete."
  info "Generated files:"
  find "${GO_OUT}" -name "*.pb.go" | sort | while read -r f; do
    info "  ${f#"${PROJECT_ROOT}/"}"
  done
}

# ---------------------------------------------------------------------------
# Kotlin compilation (placeholder)
# ---------------------------------------------------------------------------
compile_kotlin() {
  info "Compiling Kotlin lite output → ${KOTLIN_OUT#"${PROJECT_ROOT}/"}"
  mkdir -p "${KOTLIN_OUT}"

  # NOTE: The standard Android approach is to let the Gradle protobuf plugin
  # handle compilation automatically during the build.  This script provides a
  # manual fallback using a standalone protoc-gen-kotlin binary.
  #
  # Recommended Gradle setup (android/app/build.gradle.kts):
  #   plugins { id("com.google.protobuf") version "0.9.4" }
  #   protobuf {
  #     protoc { artifact = "com.google.protobuf:protoc:3.25.3" }
  #     generateProtoTasks {
  #       all().forEach { task ->
  #         task.builtins { id("kotlin") { option("lite") } }
  #         task.builtins { id("java")   { option("lite") } }
  #       }
  #     }
  #   }

  protoc \
    --proto_path="${PROTO_DIR}" \
    --kotlin_out=lite:"${KOTLIN_OUT}" \
    "${PROTO_FILES[@]}"

  success "Kotlin compilation complete."
  info "Generated files:"
  find "${KOTLIN_OUT}" -name "*.kt" | sort | while read -r f; do
    info "  ${f#"${PROJECT_ROOT}/"}"
  done
}

# ---------------------------------------------------------------------------
# Run compilations
# ---------------------------------------------------------------------------
echo ""
info "Proto source directory : ${PROTO_DIR#"${PROJECT_ROOT}/"}"
info "Project root           : ${PROJECT_ROOT}"
echo ""

FAILED=0

if ${DO_GO}; then
  compile_go || { error "Go compilation failed."; FAILED=1; }
fi

if ${DO_KOTLIN}; then
  compile_kotlin || { error "Kotlin compilation failed."; FAILED=1; }
fi

echo ""
if [[ ${FAILED} -eq 0 ]]; then
  success "All compilations succeeded."
else
  error "One or more compilations failed. See output above."
  exit 1
fi
