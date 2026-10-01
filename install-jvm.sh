#!/usr/bin/env bash
set -euo pipefail
# 所有配置统一交给工具，避免脚本覆盖现有 PATH 或 shell 配置。
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
jvm_exe="$script_dir/jvm"
args=(setup)
while (($#)); do
    case "$1" in
        --install-path)
            if (($# < 2)) || [[ -z "$2" ]]; then
                printf '%s\n' '--install-path requires a directory' >&2
                exit 1
            fi
            args+=(--path "$2")
            shift 2
            ;;
        --force) args+=(--force); shift ;;
        --system-wide)
            printf '%s\n' 'System-wide setup is not supported; use user-level setup.' >&2
            exit 1
            ;;
        --help)
            printf '%s\n' 'Usage: install-jvm.sh [--install-path PATH] [--force]'
            exit 0
            ;;
        *) printf 'Unknown option: %s\n' "$1" >&2; exit 1 ;;
    esac
done
if [[ ! -x "$jvm_exe" ]]; then
    printf '%s\n' 'Executable jvm not found beside this script.' >&2
    exit 1
fi
"$jvm_exe" "${args[@]}"
