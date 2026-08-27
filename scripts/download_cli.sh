#!/usr/bin/env sh

set -eu
umask 077

repo="tanaka-mambinge/takealot-plugin"
release_base="${TAKEALOT_RELEASE_BASE_URL:-https://github.com/${repo}/releases/latest/download}"
cache_dir="${TAKEALOT_CLI_HOME:-${HOME}/.takealot/bin}"

if ! command -v curl >/dev/null 2>&1; then
	printf '%s\n' "Takealot CLI download requires curl." >&2
	exit 1
fi

os=$(uname -s)
arch=$(uname -m)
case "${os}:${arch}" in
	Linux:x86_64|Linux:amd64) asset="takealot_linux_amd64" ;;
	Linux:aarch64|Linux:arm64) asset="takealot_linux_arm64" ;;
	Darwin:x86_64|Darwin:amd64) asset="takealot_darwin_amd64" ;;
	Darwin:arm64|Darwin:aarch64) asset="takealot_darwin_arm64" ;;
	*)
		printf '%s\n' "Unsupported Takealot CLI platform: ${os}/${arch}." >&2
		exit 1
		;;
esac

mkdir -p "${cache_dir}"
chmod 700 "${cache_dir}"
binary_tmp=$(mktemp "${cache_dir}/.takealot-cli.XXXXXX")
checksums_tmp=$(mktemp "${cache_dir}/.takealot-checksums.XXXXXX")
target="${cache_dir}/takealot"
cleanup() {
	rm -f "${binary_tmp}" "${checksums_tmp}"
}
trap cleanup EXIT HUP INT TERM

curl --fail --location --silent --show-error "${release_base}/${asset}" --output "${binary_tmp}"
curl --fail --location --silent --show-error "${release_base}/checksums.txt" --output "${checksums_tmp}"

expected=$(awk -v asset="${asset}" '$2 == asset { print tolower($1); exit }' "${checksums_tmp}")
if [ -z "${expected}" ]; then
	printf '%s\n' "No checksum found for ${asset}." >&2
	exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "${binary_tmp}" | awk '{ print tolower($1) }')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "${binary_tmp}" | awk '{ print tolower($1) }')
else
	printf '%s\n' "Takealot CLI checksum verification requires sha256sum or shasum." >&2
	exit 1
fi

if [ "${actual}" != "${expected}" ]; then
	printf '%s\n' "Checksum verification failed for ${asset}." >&2
	exit 1
fi

chmod 755 "${binary_tmp}"
mv -f "${binary_tmp}" "${target}"
printf '%s\n' "${target}"
