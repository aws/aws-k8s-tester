# FIPS TLS Compliance Test

This test validates that FIPS-enabled EKS nodes enforce FIPS-compliant TLS cipher suites when pulling container images.

## What It Does

1. Deploys two local container registries as DaemonSets on each node:
   - `registry-fips` (port 5000) — serves TLS using the node's default (FIPS-compliant) cipher suites
   - `registry-nonfips` (port 5001) — an nginx reverse proxy configured to only offer `ECDHE-RSA-CHACHA20-POLY1305`, a non-FIPS cipher
2. Seeds both registries with a test image via `skopeo`
3. Runs two test pods:
   - `test-pull-fips` — pulls from `localhost:5000` and expects success
   - `test-pull-nonfips` — pulls from `localhost:5001` and expects `ImagePullBackOff` (TLS handshake failure)

## Prerequisites

- An EKS cluster with FIPS-enabled nodes
- TLS certificates available on each node at `/mnt/server-conf/certs/`:
  - `server.crt` — server certificate
  - `server.key` — private key
- `kubeconfig` configured for the target cluster
- Go 1.21+

## Host Setup

### Amazon Linux 2023

FIPS mode must be enabled at launch time via the EKS AMI. Use a FIPS-enabled AL2023 AMI when creating the nodegroup:

```bash
# Create a FIPS-enabled nodegroup with eksctl
kubetest2 eksctl \
  --kubernetes-version=X.XX \
  --ami-family=AmazonLinux2023 \
  --up \
  --down \
  --test=exec \
  -- <test command>
```

Verify FIPS is active on a node:
```bash
# SSH into a node and check
cat /proc/sys/crypto/fips_enabled
# Expected output: 1

# Or check via sysctl
sysctl crypto.fips_enabled
# Expected output: crypto.fips_enabled = 1
```

Generate the TLS certificates on each node:
```bash
sudo mkdir -p /mnt/server-conf/certs
sudo openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /mnt/server-conf/certs/server.key \
  -out /mnt/server-conf/certs/server.crt \
  -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
```

Add the certificate to the node's trust store so containerd trusts the local registries:
```bash
sudo cp /mnt/server-conf/certs/server.crt /etc/pki/ca-trust/source/anchors/
sudo update-ca-trust
sudo systemctl restart containerd
```

Without this, containerd will reject the self-signed cert and both test pods would fail with `ImagePullBackOff`.

### Bottlerocket

Bottlerocket is an immutable OS — you can't SSH in and run `openssl` directly. Certs must be provisioned via a bootstrap container that runs before kubelet starts.

**1. Build the bootstrap container image**

The Dockerfile is minimal — it runs a user-data script at boot:

```dockerfile

FROM public.ecr.aws/docker/library/alpine:latest
RUN apk add --no-cache openssl curl
ENTRYPOINT ["/bin/sh", "/.bottlerocket/bootstrap-containers/gen-certs/user-data"]
```

Build and push to ECR:
```bash
docker build -t <your-account-id>.dkr.ecr.<region>.amazonaws.com/cert-bootstrap:v1 .
docker push <your-account-id>.dkr.ecr.<region>.amazonaws.com/cert-bootstrap:v1
```

**2. Prepare the cert generation script**

The cert generation script generates a CA + server cert, writes them to the host at `/mnt/server-conf/certs/`, and registers the CA with Bottlerocket's trust store via `apiclient`:

```bash
#!/bin/sh
set -xe

WORK_DIR=$(mktemp -d)
CERTS_DIR=/.bottlerocket/rootfs/mnt/server-conf/certs
CSR_CONF=${WORK_DIR}/csr.conf
CA_CRT=${WORK_DIR}/ca.crt
CA_KEY=${WORK_DIR}/ca.key

mkdir -p ${CERTS_DIR}

# Generate CA
openssl genrsa -out ${CA_KEY} 2048
openssl req -x509 -new -nodes -key ${CA_KEY} \
  -subj "/CN=Bottlerocket Test CA/C=US/ST=WASHINGTON/L=Seattle/O=Bottlerocket" \
  -days 1825 -out ${CA_CRT}

# Get instance metadata
TOKEN=$(curl -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
DOMAIN=$(curl -H "X-aws-ec2-metadata-token: ${TOKEN}" http://169.254.169.254/latest/meta-data/public-hostname)
IP=$(curl -H "X-aws-ec2-metadata-token: ${TOKEN}" http://169.254.169.254/latest/meta-data/public-ipv4)

# Generate CSR config with real values
cat > ${CSR_CONF} <<EOF
[ req ]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn
req_extensions = req_ext

[ dn ]
C = US
ST = WASHINGTON
L = Seattle
O = Bottlerocket
OU = Bottlerocket Dev

[ req_ext ]
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = localhost
DNS.2 = ${DOMAIN}
IP.1 = 127.0.0.1
IP.2 = ${IP}
EOF

# Generate server cert signed by CA
openssl genrsa -out ${CERTS_DIR}/server.key 2048
openssl req -new -key ${CERTS_DIR}/server.key -out ${WORK_DIR}/server.csr -config ${CSR_CONF}
openssl x509 -req -in ${WORK_DIR}/server.csr -CA ${CA_CRT} -CAkey ${CA_KEY} \
  -CAcreateserial -out ${CERTS_DIR}/server.crt -days 10000 \
  -extensions req_ext -extfile ${CSR_CONF}

# Push CA to Bottlerocket trust store
BUNDLE=$(base64 -w0 ${CA_CRT})
apiclient set pki.local-registry.data=${BUNDLE}
apiclient set pki.local-registry.trusted=true

rm -rf ${WORK_DIR}
```

Once you've created your script, you'll need to base64-encode it and set it as the value of the bootstrap container's user-data setting.

**3. Configure the bootstrap container in Bottlerocket TOML**

Add this to your Bottlerocket user data:

```toml
[settings.bootstrap-containers.gen-certs]
source = "<your-account-id>.dkr.ecr.<region>.amazonaws.com/cert-bootstrap:v1"
mode = "once"
essential = true
user-data = "<paste base64-encoded set-up-host-v2 here>"
```

- `mode = "once"` — runs only on first boot
- `essential = true` — node won't start if cert generation fails
- The script runs before kubelet, so certs are ready when pods start

**4. Launch with a FIPS Bottlerocket AMI**

```bash
kubetest2 eksctl \
  --kubernetes-version=X.XX \
  --ami-family=Bottlerocket \
  --up \
  --down \
  --test=exec \
  -- <test command>
```

Verify FIPS on Bottlerocket (via the admin container):
```bash
cat /proc/sys/crypto/fips_enabled
# Expected output: 1
```

## Running the Test

```bash
# Run all FIPS test cases
go test -tags e2e -v ./test/cases/fips/ --kubeconfig=$HOME/.kube/config

# Run a specific test case by label
go test -tags e2e -v ./test/cases/fips/ --kubeconfig=$HOME/.kube/config -labels="suite=fips"
```

Or via `kubetest2`:
```bash
kubetest2 eksctl \
  --kubernetes-version=X.XX \
  --ami-family=<AMI_Family> \
  --up \
  --down \
  --test=exec \
  -- fips.test -v
```

## Test Cases

| Test | Description | Expected Result |
|------|-------------|-----------------|
| `fips-tls-pull` | Pull image from FIPS-cipher registry (port 5000) | Pod succeeds |
| `nonfips-tls-pull` | Pull image from non-FIPS-cipher registry (port 5001) | `ImagePullBackOff` — TLS handshake rejected |
