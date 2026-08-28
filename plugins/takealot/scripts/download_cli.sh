#!/usr/bin/env sh

set -eu
umask 077

repo="tanaka-mambinge/takealot-plugin"
release_base="https://github.com/${repo}/releases/latest/download"
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
target="${cache_dir}/takealot"
version_file="${cache_dir}/takealot.version"
cached_version=""
if [ -x "${target}" ] && [ -s "${version_file}" ]; then
	cached_version=$(awk 'NR == 1 { print; exit }' "${version_file}")
fi

latest_tag=""
latest_headers=""
if [ -z "${latest_tag}" ]; then
	if latest_headers=$(curl --fail --silent --show-error --head --max-time 20 "${release_base}/${asset}" 2>/dev/null); then
		latest_tag=$(printf '%s\n' "${latest_headers}" | awk '
			BEGIN { IGNORECASE = 1 }
			/^location:/ {
				url = $0
				sub(/^[^:]*:[[:space:]]*/, "", url)
				count = split(url, parts, "/")
				for (i = 1; i < count; i++) {
					if (parts[i] == "download") {
						print parts[i + 1]
						exit
					}
				}
			}')
	fi
fi

if [ -n "${latest_tag}" ] && [ "${cached_version}" = "${latest_tag}" ] && [ -x "${target}" ]; then
	printf '%s\n' "${target}"
	exit 0
fi

if [ -z "${latest_tag}" ]; then
	printf '%s\n' "Unable to determine the latest Takealot CLI release." >&2
	exit 1
fi

binary_tmp=$(mktemp "${cache_dir}/.takealot-cli.XXXXXX")
checksums_tmp=$(mktemp "${cache_dir}/.takealot-checksums.XXXXXX")
version_tmp=$(mktemp "${cache_dir}/.takealot-version.XXXXXX")
cleanup() {
	rm -f "${binary_tmp}" "${checksums_tmp}" "${version_tmp}"
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
printf '%s\n' "${latest_tag}" >"${version_tmp}"
mv -f "${version_tmp}" "${version_file}"
printf '%s\n' "${target}"
