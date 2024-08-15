#!/usr/bin/env bash

set -eo pipefail

mkdir -p ./tmp-swagger-gen

cd proto
echo "Generate cronos swagger files"
proto_dirs=$(find ./ -path -prune -o -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
for dir in $proto_dirs; do
  # generate swagger files (filter query files)
  query_file=$(find "${dir}" -maxdepth 1 \( -name 'query.proto' -o -name 'service.proto' \))
  if [[ ! -z "$query_file" ]]; then
    echo "$query_file"
    buf generate --template buf.gen.swagger.yaml "$query_file"
  fi
done

echo "Generate cosmos swagger files"

proto_dir="../third_party"
proto_dirs=$(find "${proto_dir}/cosmos-sdk" "${proto_dir}/ibc-go" -path -prune -o -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
for dir in $proto_dirs; do
  # generate swagger files (filter query files)
  query_file=$(find "${dir}" -maxdepth 1 \( -name 'query.proto' -o -name 'service.proto' \))
  if [[ ! -z "$query_file" ]]; then
    echo "$query_file"
    buf generate --template buf.gen.swagger.yaml "$query_file"
  fi
done

buf generate --template buf.gen.swagger.yaml "buf.build/cosmos/cosmos-sdk:954f7b05f38440fc8250134b15adec47"

cd ..

echo "Combine swagger files"
# combine swagger files
# uses nodejs package `swagger-combine`.
# all the individual swagger files need to be configured in `config.json` for merging
swagger-combine ./app/docs/config.json -o ./app/docs/swagger-ui/swagger.yaml -f yaml --continueOnConflictingPaths true --includeDefinitions true

# clean swagger files
rm -rf ./tmp-swagger-gen
