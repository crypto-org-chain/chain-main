#!/bin/sh
set -e

echo 'List signing identities'
security find-identity -v -p codesigning

TMPD=$(mktemp -d)
tar xfz $1 -C "$TMPD"

echo "Codesign"
find "$TMPD" -type f \( -name "*.dylib" -or -name chain-maind \) | xargs -I{} codesign --force --options runtime -v -s "$MAC_CODE_API_DEVELOPER_ID" {}

echo "Archive"
WORKDIR=$PWD
cd "$TMPD"
# notarytool only accepts zip format
zip -r "$WORKDIR/signed.zip" .
cd $WORKDIR
tar cfz signed.tar.gz -C "$TMPD" .

echo "Notarize"
echo "$MAC_CODE_API_KEY" > /tmp/api_key.p8
UUID=$(xcrun notarytool submit signed.zip -f json --key /tmp/api_key.p8 --key-id "$MAC_CODE_API_KEY_ID" --issuer "$MAC_CODE_API_ISSUER_ID" --wait | jq -r ".id")
echo "UUID: $UUID"
xcrun notarytool log "$UUID" --key /tmp/api_key.p8 --key-id "$MAC_CODE_API_KEY_ID" --issuer "$MAC_CODE_API_ISSUER_ID"

echo "Cleanup"
rm -r "$TMPD"
rm /tmp/api_key.p8
