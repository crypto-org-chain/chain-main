let
  pkgs = import ../nix { };
  released =
    (import
      (builtins.fetchTarball "https://github.com/crypto-org-chain/chain-main/archive/v1.1.0.tar.gz")
      { }
    ).chain-maind;
  released2 =
    (import
      (builtins.fetchTarball "https://github.com/crypto-org-chain/chain-main/archive/v2.0.1.tar.gz")
      { }
    ).chain-maind;
  released3 =
    (import
      (builtins.fetchTarball "https://github.com/crypto-org-chain/chain-main/archive/v3.3.4.tar.gz")
      { }
    ).chain-maind;
  fetchFlake =
    repo: rev:
    (pkgs.flake-compat {
      src = {
        outPath = builtins.fetchTarball "https://github.com/${repo}/archive/${rev}.tar.gz";
        inherit rev;
        shortRev = builtins.substring 0 7 rev;
      };
    }).defaultNix;
  released4_2 =
    (fetchFlake "crypto-org-chain/chain-main" "b3226f06fd2a236f9957304c4d83b0ea06ed2604").default;
  released5_0 =
    (fetchFlake "crypto-org-chain/chain-main" "246ba80c2f0c7e11a0a7b483a349d177ffeb0a9d").default;
  released6_0 =
    (fetchFlake "crypto-org-chain/chain-main" "bcd5bcb30a4bac7a5939a712ac079dd631abf41b").default;
  current = pkgs.callPackage ../. { };
in
{
  # Fast CI path: v5 (genesis) → v6 → v7. Used for PR upgrade tests.
  fast = pkgs.linkFarm "upgrade-test-package" [
    {
      name = "genesis";
      path = released5_0;
    }
    {
      name = "v5.0.0";
      path = released5_0;
    }
    {
      name = "v6.0.0";
      path = released6_0;
    }
    {
      name = "v7.0.0";
      path = current;
    }
  ];

  # Full upgrade path: v1.1.0 (genesis) → ... → v7. Used for release runs.
  # To add a new protocol version: append a new entry here and update `fast`
  # above if the CI baseline should advance.
  all = pkgs.linkFarm "upgrade-test-all-package" [
    {
      name = "genesis";
      path = released;
    }
    {
      name = "v2.0.0";
      path = released2;
    }
    {
      name = "v3.0.0";
      path = released3;
    }
    {
      name = "v4.2.0";
      path = released4_2;
    }
    {
      name = "v5.0.0";
      path = released5_0;
    }
    {
      name = "v6.0.0";
      path = released6_0;
    }
    {
      name = "v7.0.0";
      path = current;
    }
  ];
}
