#!/usr/bin/env sh
# Fetch the pinned Headwater engine, verify its checksum, and extract it.
#
# Headwater types and checks the design-record shelf (DR-0021). It is a
# docs-tooling binary only: it is never linked into cmd/vendkit, and nothing on
# the consumer gate path reaches it.
#
# The pin lives here and nowhere else — version and digest move together, the
# same shape DR-0016 uses for the engine artefact. A fetch whose bytes miss the
# digest fails; there is no unverified path.
#
# Usage: tools/headwater-fetch.sh [dest-dir]   (default: .headwater/bin)
set -eu

HEADWATER_VERSION=0.1.2
HEADWATER_SHA256=69d739c4af67eab17a24283577211352c79680b2993edb65a5f8d84694bc7dd9
HEADWATER_TARGET=x86_64-unknown-linux-gnu

dest=${1:-.headwater/bin}
tarball="headwater-v${HEADWATER_VERSION}-${HEADWATER_TARGET}.tar.gz"
url="https://github.com/headwater-ai/headwater/releases/download/v${HEADWATER_VERSION}/${tarball}"

if [ -x "${dest}/headwater" ] && [ "$("${dest}/headwater" --version 2>/dev/null || true)" = "${HEADWATER_VERSION}" ]; then
  echo "headwater ${HEADWATER_VERSION} already at ${dest}/headwater"
  exit 0
fi

tmp=$(mktemp -d)
trap 'rm -rf "${tmp}"' EXIT

echo "fetching ${url}"
curl --fail --silent --show-error --location --output "${tmp}/${tarball}" "${url}"

echo "${HEADWATER_SHA256}  ${tmp}/${tarball}" | sha256sum -c -

mkdir -p "${dest}"
tar -xzf "${tmp}/${tarball}" -C "${dest}" headwater
chmod +x "${dest}/headwater"

echo "headwater ${HEADWATER_VERSION} verified and extracted to ${dest}/headwater"
