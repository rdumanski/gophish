# Deploy gophish to AWS (EC2 + Terraform)

Provisions a single EC2 instance that builds this repo's Docker image and runs
gophish behind a stable Elastic IP. The SQLite DB lives on a **separate EBS
volume** so rebuilds never wipe your campaigns.

```
            ┌──────────────── EC2 (Ubuntu 24.04, t3.medium) ────────────────┐
 targets ──▶│ :80 / :443  gophish phishing/landing server (public)          │
   you  ───▶│ :22  SSH ──tunnel──▶ :3333 admin UI (TLS, self-signed)         │
            │ docker compose (built from github.com/rdumanski/gophish)       │
            │ /opt/gophish-data ──▶ EBS data volume (SQLite, persistent)     │
            └───────────────────────────────────────────────────────────────┘
                         Elastic IP  ◀── point your domain here later
```

## What gets created

| Resource | Notes |
|---|---|
| EC2 instance | `t3.medium` (build needs >2 GB RAM), IMDSv2-only, encrypted root |
| EBS data volume | 20 GB gp3, `prevent_destroy`, holds the SQLite DB |
| Elastic IP | stable public IP for DNS later |
| Security group | 80/443 world-open; 22 to your CIDR; 3333 closed (tunnel) by default |
| Key pair | from your `ssh_public_key` |

## Prerequisites

- Terraform ≥ 1.5 and AWS credentials (`aws configure` or env vars) with EC2/EBS/EIP rights.
- An SSH key pair locally (`ssh-keygen -t ed25519` if you don't have one).
- **Your changes pushed** to the `repo_ref` you deploy (default `master`). The box
  builds from GitHub, *not* your working tree — `seed_demo.py` and the demo DB are
  gitignored and will not ship.

## Deploy

```bash
cd deploy/aws
cp terraform.tfvars.example terraform.tfvars
# edit terraform.tfvars: paste your public key, set allowed_ssh_cidr + contact_address

terraform init
terraform plan
terraform apply
```

First boot installs Docker and builds the image in-container — allow **~10–15 min**
after `apply` returns before the app answers. Watch progress:

```bash
ssh ubuntu@<public_ip> 'sudo tail -f /var/log/gophish-provision.log'
```

## First login

1. Get the auto-generated admin password (gophish prints it once, on first boot):
   ```bash
   terraform output -raw get_initial_password_command | bash
   ```
2. Open the admin tunnel and browse to the UI:
   ```bash
   $(terraform output -raw admin_tunnel_command)   # ssh -N -L 3333:localhost:3333 ubuntu@<ip>
   # then visit https://localhost:3333  (accept the self-signed cert warning)
   ```
   Log in as `admin` with the password from step 1 and change it immediately.

## Verify it's actually healthy (do this once)

```bash
ssh ubuntu@<public_ip>
cd /opt/gophish-src
sudo docker compose -f deploy/aws/docker-compose.yml ps          # gophish = Up
sudo docker compose -f deploy/aws/docker-compose.yml logs gophish # NO "permission denied" on the DB
ls -l /opt/gophish-data/gophish.db                                # exists, owned by 1000:1000
curl -I http://localhost/                                         # phishing server responds
```
A `permission denied` opening the DB means the `/opt/gophish-data` ownership didn't
take — `sudo chown -R 1000:1000 /opt/gophish-data` and restart the stack.

## Sending email (important)

AWS blocks **outbound port 25** by default, and gophish has no built-in mail server —
it sends through whatever SMTP relay you configure in a **Sending Profile** (host:port,
auth). Use a relay on 587/465 (SES SMTP, Google Workspace, your provider). Don't rely
on direct-to-MX port 25 from the box.

## Adding a domain + TLS (later)

You deployed on the raw Elastic IP. When you have a domain:

1. Point an **A record** at the Elastic IP (`terraform output public_ip`).
2. Get a cert for the phishing host (Let's Encrypt) and set `PHISH_USE_TLS=true`
   with the cert/key paths in `/opt/gophish/gophish.env`, or front it with a reverse
   proxy / ACM-backed ALB. Port 443 is already open in the security group.
3. Set the admin `trusted_origins` if you expose the admin UI on a hostname.

## Iterating without rebuilding the box

`provision.sh` is idempotent and re-runnable — to redeploy code, push to the ref then:

```bash
ssh ubuntu@<public_ip>
cd /opt/gophish-src && sudo git pull
sudo bash deploy/aws/provision.sh   # rebuilds + restarts; DB untouched
```

Editing `user_data.sh.tftpl` instead forces a **full instance replacement** on the
next `apply` (the data volume survives, the OS/container do not). Prefer the SSH path
above for routine updates.

## Costs (eu-central-1, approximate)

t3.medium ~$30/mo + 50 GB gp3 ~$4/mo + EIP (free while attached). Downsize the
instance to `t3.small` after the first successful build to roughly halve compute.

## Tear down

```bash
terraform destroy
```
The data volume has `prevent_destroy = true`; to delete it too, remove that
lifecycle block first (this erases the SQLite DB — back it up first if you care).
