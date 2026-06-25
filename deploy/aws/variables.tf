variable "aws_region" {
  description = "AWS region to deploy into."
  type        = string
  default     = "eu-central-1" # Frankfurt — closest to PL
}

variable "project_name" {
  description = "Name prefix applied to all created resources."
  type        = string
  default     = "gophish"
}

variable "instance_type" {
  description = <<-EOT
    EC2 instance type. t3.medium (4 GB) is the default because the in-container
    Go + esbuild build OOMs/thrashes on 2 GB. Runtime load is trivial; you can
    downsize to t3.small AFTER the first successful build if you want.
  EOT
  type        = string
  default     = "t3.medium"
}

variable "ssh_public_key" {
  description = <<-EOT
    Contents of your SSH public key (e.g. the text in ~/.ssh/id_ed25519.pub).
    Used to create the EC2 key pair so you can SSH in and tunnel to the admin UI.
  EOT
  type        = string
}

variable "allowed_ssh_cidr" {
  description = <<-EOT
    CIDR allowed to reach SSH (port 22). SSH is also how you reach the admin UI
    (via tunnel), so set this to your current IP, e.g. "203.0.113.4/32".
    Defaults to 0.0.0.0/0 so you are never locked out — TIGHTEN THIS.
  EOT
  type        = string
  default     = "0.0.0.0/0"
}

variable "admin_allowed_cidr" {
  description = <<-EOT
    OPTIONAL: CIDR allowed to reach the admin UI on port 3333 directly over the
    internet. Leave empty (default) to keep 3333 closed at the firewall and reach
    the admin UI only through an SSH tunnel (recommended — your IP may be dynamic).
    Set e.g. "203.0.113.4/32" to expose it directly.
  EOT
  type        = string
  default     = ""
}

variable "root_volume_size" {
  description = "Root EBS volume size in GB (holds OS, Docker images, build cache)."
  type        = number
  default     = 30
}

variable "data_volume_size" {
  description = <<-EOT
    Separate EBS data volume size in GB. Holds the SQLite DB and survives instance
    replacement (so a Terraform-triggered rebuild never wipes your campaigns).
  EOT
  type        = number
  default     = 20
}

variable "repo_url" {
  description = "Git URL the instance clones and builds from."
  type        = string
  default     = "https://github.com/rdumanski/gophish.git"
}

variable "repo_ref" {
  description = "Git branch/tag/ref to build. The box builds THIS, not your local tree — push first."
  type        = string
  default     = "master"
}

variable "contact_address" {
  description = <<-EOT
    Abuse-contact email baked into config.contact_address. Good hygiene for
    phishing infrastructure on AWS — set it to a mailbox you monitor.
  EOT
  type        = string
  default     = ""
}
