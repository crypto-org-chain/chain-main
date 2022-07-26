# Advanced TMKMS Integration

- The default consensus engine available within the SDK is Tendermint Core. See [Tendermint notes on running in production](https://docs.tendermint.com/master/tendermint-core/running-in-production.html) and [notes on setting up a validator](https://docs.tendermint.com/master/nodes/validators.html#setting-up-a-validator)
- Validator block signing should be via [tmkms](https://github.com/iqlusioninc/tmkms)

## Setting up AWS Nitro Enclaves + Tendermint KMS for signing blocks
::: warning CAUTION
The setup isn't yet ready for production use: 
1) It is not yet audited
2) The [tmkms prototype fork](https://github.com/crypto-com/tmkms-light) isn't meant to be maintained in the long term
:::

### Background
[TMKMS](https://github.com/iqlusioninc/tmkms), initially targeting Cosmos Validators, provide **High-availability**, **Double-signing prevention** and **Hardware security module**.

Currently, TMKMS provides both **hardware signing** and **softsign**.
However, it is hard or impossible to plug your own [Hardware Security Modules(HSM)](https://github.com/iqlusioninc/tmkms#hardware-security-modules-recommended) to the major cloud providers when one wants to run it on the cloud for **hardware signing**. On the other hand, it is also insecure to use **softsign** as your generated signing key is actually in plain text on the machine.

What we want to achieve is just running TMKMS securely and provision validator conveniently on the cloud. To meet this end, we now can leverage [AWS Nitro Enclaves](https://aws.amazon.com/blogs/aws/aws-nitro-enclaves-isolated-ec2-environments-to-process-confidential-data/) to execute TMKMS and TMKMS then decrypts (during initialization) the signing via [AWS KMS](https://aws.amazon.com/kms/). Read more details [here](https://github.com/tomtau/tmkms/blob/feature/nitro-enclave/README.nitro.md)

Note that this is still work in progress and this document only describes a basic setup, so it is not yet ready for the production use. We recommend looking at other materials for additional setups, such as the [Security best practices for AWS KMS whitepaper](https://d0.awsstatic.com/whitepapers/aws-kms-best-practices.pdf).

![](./assets/tmkms_vsock_enclave.png)

### Step 1. Set up supported EC2 instance types
Virtualized Nitro-based instances with at least four vCPUs. t3, t3a, t4g, a1, c6g, c6gd, m6g, m6gd, r6g, and r6gd instances are not supported.

We recommend `m5a.xlarge` and `Amazon Linux 2 AMI` for easier installation for AWS Nitro Enclaves CLI.

- Remember to check `Enable` for Enclave in `Advanced Details` when configuring instance details.
![](./assets/aws_enclave_ec2_details.png)

### Step 2. Installing the Nitro Enclaves CLI
One needs to install Docker + Nitro Enclaves CLI.

Follow this [doc](https://docs.aws.amazon.com/enclaves/latest/user/nitro-enclave-cli-install.html) to proceed.
### Step 3. Prepare TMKMS Enclave images on EC2
You can either follow this [compiling-tmkms-for-aws-ne](https://github.com/tomtau/tmkms/blob/feature/nitro-enclave/README.nitro.md#compiling-tmkms-for-aws-ne) to build **TMKMS Enclave images** from scratch or simply use our published [image](https://hub.docker.com/r/cryptocom/nitro-enclave-tmkms/tags).

```bash
$ mkdir ~/.tmkms
$ nitro-cli build-enclave --docker-uri cryptocom/nitro-enclave-tmkms:latest --output-file ~/.tmkms/tmkms.eif
```
After building the enclave image, you should obtain 3 [enclave's measurements(PCRs)](https://docs.aws.amazon.com/enclaves/latest/user/set-up-attestation.html#where): PCR0 (SHA384 hash of the image), PCR1 (SHA384 hash of the OS kernel and the bootstrap process), and PCR2 (SHA384 hash of the application). Take a note of the **PCR0** value.
One can also use PCR3 and PCR8, for more details, please find this [link](https://docs.aws.amazon.com/enclaves/latest/user/enclaves-user.pdf)

And also create and take a note of **PCR4** manually which is unique across ec2.
```bash
$ printf "PCR4: %s\n" $(INSTANCE_ID="$(curl http://169.254.169.254/latest/meta-data/instance-id -s)"; python -c"import hashlib, sys; h=hashlib.sha384(); h.update(b'\0'*48); h.update(\"$INSTANCE_ID\".encode('utf-8')); print(h.hexdigest())")
```

### Step 4. Preparing IAM instance role and AWS KMS policy
#### Step 4.1. Create an IAM role for EC2
[Create an IAM role](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html#create-iam-role)for the created EC2 previously without permissions policies attached.
We will allow this role to decrypt with CMK inside nitro enclave in [KMS key policy](https://docs.aws.amazon.com/kms/latest/developerguide/key-policies.html) instead.

Attach this role to the previously created EC2. Check this [guide](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html#attach-iam-role).

#### Step 4.2. Create your CMK

- Create your [symmetric CMK](https://docs.aws.amazon.com/kms/latest/developerguide/create-keys.html#create-symmetric-cmk)

- Define key administrative permissions and key usage permissions that user can admin, encrypt and decrypt the signing key in your local or a trusted machine via [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2-mac.html).

![](./assets/aws_kms_admin.png)

- Edit key policy to allow only TMKMS inside nitro enclave to decrypt instead of entire EC2 and encrypt on EC2
  You should have a generated policy shown in the console.

  For the decryption action, you should add the following snippet in "Statement" as:
```json
{
    "Id": "key-consolepolicy-3",
    "Version": "2012-10-17",
    "Statement": [
        ...
        {
            "Sid": "Enable decrypt from nitro enclave only",
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::<AWS_ACCOUNT_ID>:role/<EC2_IAM_ROLE>"
            },
            "Action": "kms:Decrypt",
            "Resource": "*",
            "Condition": {
                "StringEqualsIgnoreCase": {
                    "kms:RecipientAttestation:PCR4": "<PCR4>",
                    "kms:RecipientAttestation:PCR0": "<PCR0>"
                }
            }
       	},
        {
            "Sid": "Enable encrypt from instance only",
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::<AWS_ACCOUNT_ID>:role/<EC2_IAM_ROLE>"
            },
            "Action": "kms:Encrypt",
            "Resource": "*"
        }
    ]
}
```
  **Change `EC2_IAM_ROLE` ,`PCR0` and `PCR4` to what we just created in previous steps.**

If you plan to run the tmkms enclave in the debug mode, set the recipient attestation value to: `000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000` (instead of the PCR0 value).

### Step 5. Prepare encrypted validator signing key on your local machine

#### Step 5.1. Install tmkms-nitro-helper on EC2

Install tmkms-nitro-helper from source code.
```bash
$ git clone https://github.com/crypto-com/tmkms-light.git && cd tmkms-light
$ sudo yum install -y openssl-devel
$ cargo build --release -p tmkms-nitro-helper
$ cp ./target/release/tmkms-nitro-helper /usr/local/bin/
```

#### Step 5.2. Generate a new encrypted validator signing key

`bech32-prefix` is `crocnclconspub` for mainnet and `tcrocnclconspub` for testnet

```bash
$ tmkms-nitro-helper init -a <KMS_REGION> -k <KMS_KEY_ID> -p bech32 -b <bech32-prefix>
```
It should generate a `bech32 public key` to stdout and an encrypted private key in relative path `secrets/secret.key`. We need bech32 public key for node join and secret.key to decrypt inside enclave.
### Step 6. Configure tmkms.toml for enclave TMKMS on EC2

Move above generated `secrets/secret.key` to `~/.tmkms` directory

Create `tmkms.toml` under `~/.tmkms` directory as:

```toml
address = 'unix:///tmp/sockets/validator.socket'
chain_id = '<chain id>'
sealed_consensus_key_path = 'secrets/secret.key'
state_file_path = 'state/priv_validator_state.json'
enclave_config_cid = 15 #overridden by flag
enclave_config_port = 5050
enclave_state_port = 5555
enclave_tendermint_conn = 5000
aws_region = '<AWS region to use for KMS>'
```

:::details Example: tmkms.toml for testnet

```toml
address = 'unix:///home/ec2-user/sockets/validator.socket'
chain_id = 'testnet-croeseid-4'
sealed_consensus_key_path = '/home/ec2-user/.tmkms/secrets/secret.key'
state_file_path = '/home/ec2-user/.tmkms/state/priv_validator_state.json'
enclave_config_cid = 15 #overridden by flag
enclave_config_port = 5050
enclave_state_port = 5555
enclave_tendermint_conn = 5000
aws_region = 'ap-southeast-1'
```

:::

### Step 7. Create TMKMS enclave service
To launch the TMKMS enclave, one needs to execute several commands to make it work.

```bash
$ nitro-cli run-enclave --cpu-count 2 --memory 512 --eif-path /home/ec2-user/.tmkms/tmkms.eif
# run in background with specific kms region
$ vsock-proxy 8000 kms.<KMS_REGION>.amazonaws.com 443 & 
$ tmkms-nitro-helper start -c /home/ec2-user/.tmkms/tmkms.toml --cid $(nitro-cli describe-enclaves | jq -r .[0].EnclaveCID)
```

In order to have a resilient validator, one should run the TMKMS enclave as a service.

#### Step 7.1. Create a script to run TMKMS enclave

```bash
#!/bin/bash

set -e

TRAP_FUNC ()
{
  nitro-cli terminate-enclave --enclave-id $(nitro-cli describe-enclaves | jq -r .[0].EnclaveID) || echo "no existing enclave"
  sudo kill -TERM $(pidof vsock-proxy)
  exit 1
}

nitro-cli run-enclave --cpu-count 2 --memory 512 --eif-path /home/ec2-user/.tmkms/tmkms.eif || TRAP_FUNC

vsock-proxy 8000 kms.<KMS_REGION>.amazonaws.com 443 &
echo "[vsock-proxy] Running in background ..."

trap TRAP_FUNC TERM INT SIGKILL

sleep 1
/home/ec2-user/bin/tmkms-nitro-helper start -c /home/ec2-user/.tmkms/tmkms.toml --cid $(nitro-cli describe-enclaves | jq -r .[0].EnclaveCID)

```
One should adjust the `<KMS_REGION>` in the script if set differently in `tmkms.toml` eg. `us-east-1`

Create the script `run_tmkms_nitro_helper.sh` with executable permissions under `~/.tmkms` directory
```bash
$ chmod +x run_tmkms_nitro_helper.sh
```

#### Step 7.2. Create systemd service for TMKMS enclave

```toml
[Unit]
Description=Tendermint KMS
ConditionPathExists=/home/ec2-user/.tmkms/tmkms.eif
After=network.target

[Service]
Type=simple
User=ec2-user
Group=ec2-user
LimitNOFILE=1024

Restart=on-failure
RestartSec=10

WorkingDirectory=/home/ec2-user/.tmkms

# make sure log directory exists
PermissionsStartOnly=true

ExecStartPre=/bin/mkdir -p /home/ec2-user/sockets /home/ec2-user/state
ExecStartPre=/bin/chown ec2-user:ec2-user /home/ec2-user/sockets /home/ec2-user/state
ExecStart=/home/ec2-user/.tmkms/run_tmkms_nitro_helper.sh

[Install]
WantedBy=multi-user.target
```
One should adjust the path in the systemd file if set different paths for the binary and script.

Create `/lib/systemd/system/tmkms.service` and run the service

```bash
sudo systemctl daemon-reload
sudo systemctl enable tmkms.service
sudo systemctl start tmkms.service
```

### Step 8. Running chain-maind

One should follow the same steps in [Croeseid Testnet: Running Nodes](./croeseid-testnet.md)

Except for one last thing one needs to further configure `~/.chain-maind/config/config.toml` to enable enclave tmkm to sign.

In `~/.chain-maind/config/config.toml`, `priv_validator_key_file` and `priv_validator_state_file` should be commented and uncomment `priv_validator_laddr` to value `unix://...` which should match the `address` in `tmkms.toml`. e.g. `unix:///home/ec2-user/sockets/validator.socket`
